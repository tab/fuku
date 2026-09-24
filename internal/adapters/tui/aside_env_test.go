package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_AsideEnvTab(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)

	theme := terminal.NewTheme(terminal.AppearanceDark)

	service := &model.Service{ID: "id-api", Name: "api", Status: model.StatusStopped}

	tests := []struct {
		name        string
		before      func() Model
		wantContain []string
		wantMissing []string
	}{
		{
			name: "empty cache shows availability message",
			before: func() Model {
				mockEnvironment.EXPECT().Env("id-api").Return(nil)

				m := Model{theme: theme, environment: mockEnvironment}
				m.state.asideTab = AsideTabEnv

				return m
			},
			wantContain: []string{
				"no environment variables available",
			},
			wantMissing: []string{
				"APP_NAME",
			},
		},
		{
			name: "entries render under environment section",
			before: func() Model {
				mockEnvironment.EXPECT().Env("id-api").Return([]model.Env{
					{Key: "APP_NAME", Value: "hub-api"},
					{Key: "JWT_SECRET", Value: "dev-wins"},
				})

				m := Model{theme: theme, environment: mockEnvironment}
				m.state.asideTab = AsideTabEnv

				return m
			},
			wantContain: []string{
				"environment",
				"APP_NAME", "hub-api",
				"JWT_SECRET", "dev-wins",
			},
			wantMissing: []string{
				"no environment variables available",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.asideContent(service, 80)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			for _, miss := range tt.wantMissing {
				assert.NotContains(t, result, miss)
			}
		})
	}
}

func Test_AsideEnvTab_WrapsLongValueWithoutTruncation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)
	mockEnvironment.EXPECT().Env("id-api").Return([]model.Env{
		{Key: "URL", Value: strings.Repeat("x", 200)},
	})

	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark), environment: mockEnvironment}
	m.state.asideTab = AsideTabEnv

	service := &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning}

	result := m.asideContent(service, 40)

	assert.Equal(t, 200, strings.Count(result, "x"), "every value rune must be present after hard-wrap")
	assert.Equal(t, 0, strings.Count(result, "…"), "wrapped values must never be truncated with an ellipsis")
	assert.Greater(t, strings.Count(result, "\n"), 1, "long value must span multiple lines")
}

func Test_AsideEnvTab_TinyInnerWidthReturnsEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)
	mockEnvironment.EXPECT().Env("id-api").Return([]model.Env{
		{Key: "A", Value: "v"},
	})

	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark), environment: mockEnvironment}

	service := &model.Service{ID: "id-api"}

	result := m.asideEnvTab(service, 4)

	assert.Empty(t, result)
}

func Test_WrapRunes(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		firstWidth int
		width      int
		want       []string
	}{
		{
			name:       "empty input returns nil",
			in:         "",
			firstWidth: 10,
			width:      10,
			want:       nil,
		},
		{
			name:       "zero first width returns nil",
			in:         "abc",
			firstWidth: 0,
			width:      10,
			want:       nil,
		},
		{
			name:       "zero continuation width returns nil",
			in:         "abc",
			firstWidth: 10,
			width:      0,
			want:       nil,
		},
		{
			name:       "value fits in first chunk",
			in:         "abc",
			firstWidth: 10,
			width:      10,
			want:       []string{"abc"},
		},
		{
			name:       "first chunk smaller than continuation",
			in:         "abcdefghij",
			firstWidth: 3,
			width:      4,
			want:       []string{"abc", "defg", "hij"},
		},
		{
			name:       "multi-byte runes counted by rune not byte",
			in:         "αβγδεζηθ",
			firstWidth: 3,
			width:      3,
			want:       []string{"αβγ", "δεζ", "ηθ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapRunes(tt.in, tt.firstWidth, tt.width)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_EnvWrappedRow(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}

	tests := []struct {
		name       string
		row        cardRow
		labelWidth int
		available  int
		wantLines  int
		wantSubstr []string
	}{
		{
			name:       "value fits beside label on one line",
			row:        cardRow{label: "API", value: "abc"},
			labelWidth: 5,
			available:  40,
			wantLines:  1,
			wantSubstr: []string{"API", "abc"},
		},
		{
			name:       "long value wraps to continuation lines",
			row:        cardRow{label: "K", value: "abcdefghij"},
			labelWidth: 3,
			available:  8,
			wantLines:  2,
			wantSubstr: []string{"K", "abcde", "fghij"},
		},
		{
			name:       "label wider than available row returns label only",
			row:        cardRow{label: "VERY_LONG_LABEL", value: "v"},
			labelWidth: 15,
			available:  10,
			wantLines:  1,
			wantSubstr: []string{"VERY_LONG_LABEL"},
		},
		{
			name:       "empty value returns label block only",
			row:        cardRow{label: "K", value: ""},
			labelWidth: 5,
			available:  40,
			wantLines:  1,
			wantSubstr: []string{"K"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := m.envWrappedRow(tt.row, tt.labelWidth, tt.available)

			assert.Len(t, lines, tt.wantLines)

			joined := strings.Join(lines, "\n")
			for _, want := range tt.wantSubstr {
				assert.Contains(t, joined, want)
			}
		})
	}
}
