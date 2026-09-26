package doctor

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Runner_environmentSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)
	mockEnvironment.EXPECT().Getenv("SHELL").Return("/bin/zsh")
	mockEnvironment.EXPECT().Getenv("LANG").Return("en_US.UTF-8")
	mockEnvironment.EXPECT().Executable().Return("/usr/local/bin/fuku", nil)
	mockEnvironment.EXPECT().PathExecutable().Return("/usr/local/bin/fuku", nil)

	subject := NewRunner(Options{}, model.Config{}, mockEnvironment, nil, nil, nil)

	section := subject.environmentSection()

	assert.Equal(t, "Environment", section.Title)
	require.Len(t, section.Results, 3)
	assert.Equal(t, model.CheckSystem, section.Results[0].ID)
	assert.Equal(t, model.CheckRuntime, section.Results[1].ID)
	assert.Equal(t, model.CheckInstall, section.Results[2].ID)
}

func Test_Runner_checkSystem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)

	subject := NewRunner(Options{}, model.Config{}, mockEnvironment, nil, nil, nil)

	tests := []struct {
		name     string
		before   func()
		expected []model.Detail
	}{
		{
			name: "reports shell and locale",
			before: func() {
				mockEnvironment.EXPECT().Getenv("SHELL").Return("/bin/zsh")
				mockEnvironment.EXPECT().Getenv("LANG").Return("en_US.UTF-8")
			},
			expected: []model.Detail{
				{Key: "os", Value: runtime.GOOS},
				{Key: "arch", Value: runtime.GOARCH},
				{Key: "shell", Value: "/bin/zsh"},
				{Key: "LANG", Value: "en_US.UTF-8"},
			},
		},
		{
			name: "unset locale shows a dash",
			before: func() {
				mockEnvironment.EXPECT().Getenv("SHELL").Return("")
				mockEnvironment.EXPECT().Getenv("LANG").Return("")
			},
			expected: []model.Detail{
				{Key: "os", Value: runtime.GOOS},
				{Key: "arch", Value: runtime.GOARCH},
				{Key: "shell", Value: ""},
				{Key: "LANG", Value: "-"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkSystem()

			assert.Equal(t, model.CheckSystem, r.ID)
			assert.Equal(t, model.SeverityOK, r.Severity)
			assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, r.Summary)
			assert.Equal(t, tt.expected, r.Details)
		})
	}
}

func Test_checkRuntime(t *testing.T) {
	r := checkRuntime()

	assert.Equal(t, model.CheckRuntime, r.ID)
	assert.Equal(t, model.SeverityOK, r.Severity)
	assert.Equal(t, runtime.Version(), r.Summary)
	assert.Equal(t, []model.Detail{
		{Key: "go version", Value: runtime.Version()},
		{Key: "GOOS", Value: runtime.GOOS},
		{Key: "GOARCH", Value: runtime.GOARCH},
	}, r.Details)
}

func Test_Runner_checkInstall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)

	subject := NewRunner(Options{}, model.Config{}, mockEnvironment, nil, nil, nil)

	tests := []struct {
		name                string
		before              func()
		expectedSeverity    model.Severity
		expectedSummary     string
		expectedDetails     []model.Detail
		expectedRemediation string
	}{
		{
			name: "executable on PATH",
			before: func() {
				mockEnvironment.EXPECT().Executable().Return("/usr/local/bin/fuku", nil)
				mockEnvironment.EXPECT().PathExecutable().Return("/usr/local/bin/fuku", nil)
			},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "installation looks consistent",
			expectedDetails: []model.Detail{
				{Key: "executable", Value: "/usr/local/bin/fuku"},
				{Key: "PATH fuku", Value: "/usr/local/bin/fuku"},
			},
		},
		{
			name: "executable off PATH",
			before: func() {
				mockEnvironment.EXPECT().Executable().Return("/home/dev/fuku", nil)
				mockEnvironment.EXPECT().PathExecutable().Return("", assert.AnError)
			},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "installation looks consistent",
			expectedDetails: []model.Detail{
				{Key: "executable", Value: "/home/dev/fuku"},
			},
		},
		{
			name: "executable unresolved",
			before: func() {
				mockEnvironment.EXPECT().Executable().Return("", assert.AnError)
			},
			expectedSeverity:    model.SeverityWarn,
			expectedSummary:     "could not resolve fuku executable",
			expectedRemediation: "ensure fuku binary is reachable on PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkInstall()

			assert.Equal(t, model.CheckInstall, r.ID)
			assert.Equal(t, tt.expectedSeverity, r.Severity)
			assert.Equal(t, tt.expectedSummary, r.Summary)
			assert.Equal(t, tt.expectedDetails, r.Details)
			assert.Equal(t, tt.expectedRemediation, r.Remediation)
		})
	}
}
