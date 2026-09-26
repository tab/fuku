package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Doctor_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockChecker := NewMockChecker(ctrl)
	mockRenderer := NewMockRenderer(ctrl)

	var stdout bytes.Buffer

	subject := NewDoctor(mockChecker, mockRenderer, &stdout)

	healthy := &model.Report{Sections: []model.Section{{Results: []model.Result{{Severity: model.SeverityWarn}}}}}
	failing := &model.Report{Sections: []model.Section{{Results: []model.Result{{Severity: model.SeverityFail}}}}}

	tests := []struct {
		name         string
		before       func()
		expectedExit int
		expectedErr  error
	}{
		{
			name: "renders a healthy report and exits 0",
			before: func() {
				mockChecker.EXPECT().Run(gomock.Any()).Return(healthy)
				mockRenderer.EXPECT().Render(&stdout, healthy).Return(nil)
			},
			expectedExit: 0,
		},
		{
			name: "renders a failing report and exits 2",
			before: func() {
				mockChecker.EXPECT().Run(gomock.Any()).Return(failing)
				mockRenderer.EXPECT().Render(&stdout, failing).Return(nil)
			},
			expectedExit: 2,
		},
		{
			name: "render failure exits 3 with the error",
			before: func() {
				mockChecker.EXPECT().Run(gomock.Any()).Return(healthy)
				mockRenderer.EXPECT().Render(&stdout, healthy).Return(assert.AnError)
			},
			expectedExit: 3,
			expectedErr:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exit, err := subject.Run(t.Context())

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exit)
		})
	}
}

func Test_exitCode(t *testing.T) {
	tests := []struct {
		name     string
		results  []model.Result
		expected int
	}{
		{
			name:     "all ok",
			results:  []model.Result{{Severity: model.SeverityOK}},
			expected: 0,
		},
		{
			name:     "with warn but no fail",
			results:  []model.Result{{Severity: model.SeverityWarn}},
			expected: 0,
		},
		{
			name:     "with fail",
			results:  []model.Result{{Severity: model.SeverityFail}},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &model.Report{Sections: []model.Section{{Results: tt.results}}}

			assert.Equal(t, tt.expected, exitCode(report))
		})
	}
}
