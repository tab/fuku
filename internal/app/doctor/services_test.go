package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Runner_servicesSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFilesystem := NewMockFilesystem(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, mockFilesystem, nil, nil)

	api := &model.Service{Name: "api", Directory: "/srv/api", Environment: &model.EnvFiles{}}

	tests := []struct {
		name               string
		before             func()
		state              *state
		expectedNote       string
		expectedSeverities []model.Severity
	}{
		{
			name:               "config did not load",
			before:             func() {},
			state:              &state{Config: model.Config{Error: assert.AnError}},
			expectedNote:       "skipped (config did not load)",
			expectedSeverities: []model.Severity{model.SeverityIdle, model.SeverityIdle, model.SeverityIdle},
		},
		{
			name:               "profile did not resolve",
			before:             func() {},
			state:              &state{profileErr: assert.AnError},
			expectedNote:       "skipped (profile did not resolve)",
			expectedSeverities: []model.Severity{model.SeverityIdle, model.SeverityIdle, model.SeverityIdle},
		},
		{
			name: "resolves to services",
			before: func() {
				mockFilesystem.EXPECT().DirExists("/srv/api").Return(true)
			},
			state:              &state{Options: Options{Profile: model.ProfileDefault}, services: []*model.Service{api}},
			expectedNote:       "active profile: default · 1 services",
			expectedSeverities: []model.Severity{model.SeverityOK, model.SeverityIdle, model.SeverityIdle},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			section := subject.servicesSection(tt.state)

			assert.Equal(t, "Services", section.Title)
			assert.Equal(t, tt.expectedNote, section.Note)
			require.Len(t, section.Results, 3)
			assert.Equal(t, model.CheckServicesDirectories, section.Results[0].ID)
			assert.Equal(t, model.CheckServicesDotenv, section.Results[1].ID)
			assert.Equal(t, model.CheckServicesReadiness, section.Results[2].ID)
			assert.Equal(t, tt.expectedSeverities, []model.Severity{section.Results[0].Severity, section.Results[1].Severity, section.Results[2].Severity})
		})
	}
}

func Test_Runner_checkServiceDirectories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFilesystem := NewMockFilesystem(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, mockFilesystem, nil, nil)

	api := &model.Service{Name: "api", Directory: "api"}
	missing := &model.Service{Name: "missing", Directory: "missing"}

	tests := []struct {
		name            string
		before          func()
		services        []*model.Service
		expected        model.Severity
		expectedDetails []model.Detail
	}{
		{
			name: "all present",
			before: func() {
				mockFilesystem.EXPECT().Getwd().Return("/home/dev/project", nil)
				mockFilesystem.EXPECT().DirExists("/home/dev/project/api").Return(true)
			},
			services: []*model.Service{api},
			expected: model.SeverityOK,
			expectedDetails: []model.Detail{
				{Key: "api", Value: "/home/dev/project/api"},
			},
		},
		{
			name: "some missing",
			before: func() {
				mockFilesystem.EXPECT().Getwd().Return("/home/dev/project", nil).Times(2)
				mockFilesystem.EXPECT().DirExists("/home/dev/project/api").Return(true)
				mockFilesystem.EXPECT().DirExists("/home/dev/project/missing").Return(false)
			},
			services: []*model.Service{api, missing},
			expected: model.SeverityWarn,
			expectedDetails: []model.Detail{
				{Key: "api", Value: "/home/dev/project/api"},
				{Key: "missing", Value: "/home/dev/project/missing (MISSING)"},
			},
		},
		{
			name: "unknown working directory keeps the relative path",
			before: func() {
				mockFilesystem.EXPECT().Getwd().Return("", assert.AnError)
				mockFilesystem.EXPECT().DirExists("api").Return(true)
			},
			services: []*model.Service{api},
			expected: model.SeverityOK,
			expectedDetails: []model.Detail{
				{Key: "api", Value: "api"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkServiceDirectories(tt.services)

			assert.Equal(t, model.CheckServicesDirectories, r.ID)
			assert.Equal(t, tt.expected, r.Severity)
			assert.Equal(t, tt.expectedDetails, r.Details)
		})
	}
}

func Test_Runner_checkServiceDotenv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFilesystem := NewMockFilesystem(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, mockFilesystem, nil, nil)

	api := &model.Service{Name: "api", Directory: "/srv/api", Environment: &model.EnvFiles{Files: []string{".env"}}}
	missing := &model.Service{Name: "missing", Directory: "/srv/api", Environment: &model.EnvFiles{Files: []string{".env.local"}}}
	noenv := &model.Service{Name: "noenv", Directory: "/srv/api", Environment: &model.EnvFiles{}}
	defaulted := &model.Service{Name: "defaulted", Directory: "/srv/api", Environment: &model.EnvFiles{Files: []string{".env"}, Defaulted: true}}

	tests := []struct {
		name     string
		before   func()
		services []*model.Service
		expected model.Severity
	}{
		{
			name:     "no env files referenced",
			before:   func() {},
			services: []*model.Service{noenv},
			expected: model.SeverityIdle,
		},
		{
			name:     "defaulted env files are not checked",
			before:   func() {},
			services: []*model.Service{defaulted},
			expected: model.SeverityIdle,
		},
		{
			name: "all present",
			before: func() {
				mockFilesystem.EXPECT().FileExists("/srv/api/.env").Return(true)
			},
			services: []*model.Service{api},
			expected: model.SeverityOK,
		},
		{
			name: "some missing",
			before: func() {
				mockFilesystem.EXPECT().FileExists("/srv/api/.env").Return(true)
				mockFilesystem.EXPECT().FileExists("/srv/api/.env.local").Return(false)
			},
			services: []*model.Service{api, missing},
			expected: model.SeverityWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkServiceDotenv(tt.services)

			assert.Equal(t, model.CheckServicesDotenv, r.ID)
			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}

func Test_checkServiceReadiness(t *testing.T) {
	http := &model.Service{Name: "http", Readiness: &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}}
	tcp := &model.Service{Name: "tcp", Readiness: &model.Readiness{Type: model.ReadinessTCP, Address: "localhost:5432"}}
	log := &model.Service{Name: "log", Readiness: &model.Readiness{Type: model.ReadinessLog, Pattern: `ready\s+\d+`}}
	badregex := &model.Service{Name: "badregex", Readiness: &model.Readiness{Type: model.ReadinessLog, Pattern: `[invalid`}}
	badaddr := &model.Service{Name: "badaddr", Readiness: &model.Readiness{Type: model.ReadinessTCP, Address: "no-port"}}
	urlnoscheme := &model.Service{Name: "urlnoscheme", Readiness: &model.Readiness{Type: model.ReadinessHTTP, URL: "localhost:8080/health"}}
	urlnohost := &model.Service{Name: "urlnohost", Readiness: &model.Readiness{Type: model.ReadinessHTTP, URL: "http:///health"}}
	urlwrongscheme := &model.Service{Name: "urlwrongscheme", Readiness: &model.Readiness{Type: model.ReadinessHTTP, URL: "ftp://host/health"}}
	none := &model.Service{Name: "none"}

	tests := []struct {
		name     string
		services []*model.Service
		expected model.Severity
	}{
		{
			name:     "no probes",
			services: []*model.Service{none},
			expected: model.SeverityIdle,
		},
		{
			name:     "all probes parse",
			services: []*model.Service{http, tcp, log},
			expected: model.SeverityOK,
		},
		{
			name:     "bad regex",
			services: []*model.Service{badregex},
			expected: model.SeverityFail,
		},
		{
			name:     "bad address",
			services: []*model.Service{badaddr},
			expected: model.SeverityFail,
		},
		{
			name:     "http url without scheme is rejected",
			services: []*model.Service{urlnoscheme},
			expected: model.SeverityFail,
		},
		{
			name:     "http url with empty host is rejected",
			services: []*model.Service{urlnohost},
			expected: model.SeverityFail,
		},
		{
			name:     "http url with non-http scheme is rejected",
			services: []*model.Service{urlwrongscheme},
			expected: model.SeverityFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := checkServiceReadiness(tt.services)

			assert.Equal(t, model.CheckServicesReadiness, r.ID)
			assert.Equal(t, tt.expected, r.Severity)
		})
	}
}

func Test_validateHTTPURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "valid http",
			input: "http://localhost:8080/health",
		},
		{
			name:  "valid https",
			input: "https://example.com/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPURL(tt.input)

			require.NoError(t, err)
		})
	}
}

func Test_validateHTTPURL_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "missing scheme",
			input:    "localhost:8080/health",
			expected: "scheme must be http or https",
		},
		{
			name:     "empty host",
			input:    "http:///health",
			expected: "missing host",
		},
		{
			name:     "wrong scheme",
			input:    "ftp://host/health",
			expected: "scheme must be http or https",
		},
		{
			name:     "parse error",
			input:    "://broken",
			expected: `parse "://broken": missing protocol scheme`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPURL(tt.input)

			require.EqualError(t, err, tt.expected)
		})
	}
}
