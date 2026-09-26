package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Runner_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEnvironment := NewMockEnvironment(ctrl)
	mockFilesystem := NewMockFilesystem(ctrl)
	mockProfiles := NewMockProfiles(ctrl)
	mockRuntime := NewMockRuntime(ctrl)

	api := model.Service{Name: "api", Command: "make run", Directory: "api", Tier: model.TierDefault, Environment: &model.EnvFiles{}}

	project := model.Project{
		Services:    []model.Service{api},
		Profiles:    map[string]model.Profile{model.ProfileDefault: {All: true}},
		Logging:     model.Logging{Level: "info", Format: "console"},
		Concurrency: model.Concurrency{Workers: 5},
		Retry:       model.Retry{Attempts: 3, Backoff: 500 * time.Millisecond},
		Logs:        model.Logs{Buffer: 1000, History: 5000},
	}

	topology := model.Topology{
		Order:        []string{model.TierDefault},
		TierServices: map[string][]string{model.TierDefault: {"api"}},
	}

	tiers := []model.Tier{{Name: model.TierDefault, Services: []*model.Service{&api}}}

	options := Options{Profile: model.ProfileDefault, Fingerprint: "0123456789abcdef", Version: "0.99.0"}

	rows := []model.CheckID{
		model.CheckSystem, model.CheckRuntime, model.CheckInstall,
		model.CheckConfigFile, model.CheckConfigOverride, model.CheckConfigValidate, model.CheckConfigSettings,
		model.CheckServicesDirectories, model.CheckServicesDotenv, model.CheckServicesReadiness,
		model.CheckTopologyTiers, model.CheckTopologyProfile,
		model.CheckRuntimeInstance, model.CheckRuntimeSockets, model.CheckRuntimePorts,
	}

	tests := []struct {
		name             string
		before           func()
		config           model.Config
		expectedSections []string
		expectedTally    model.Tally
		expectedFile     model.Severity
	}{
		{
			name: "no config",
			before: func() {
				mockEnvironment.EXPECT().Getenv("SHELL").Return("/bin/zsh")
				mockEnvironment.EXPECT().Getenv("LANG").Return("en_US.UTF-8")
				mockEnvironment.EXPECT().Executable().Return("/usr/local/bin/fuku", nil)
				mockEnvironment.EXPECT().PathExecutable().Return("/usr/local/bin/fuku", nil)
				mockProfiles.EXPECT().Resolve(model.ProfileDefault).Return(nil, contracts.ErrProfileNotFound)
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock"})
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock"})
			},
			config:           model.Config{},
			expectedSections: []string{"Environment", "Configuration", "Services", "Topology", "Runtime"},
			expectedTally:    model.Tally{OK: 6, Idle: 7, Fail: 2},
			expectedFile:     model.SeverityFail,
		},
		{
			name: "minimal config",
			before: func() {
				mockEnvironment.EXPECT().Getenv("SHELL").Return("/bin/zsh")
				mockEnvironment.EXPECT().Getenv("LANG").Return("")
				mockEnvironment.EXPECT().Executable().Return("/usr/local/bin/fuku", nil)
				mockEnvironment.EXPECT().PathExecutable().Return("", assert.AnError)
				mockFilesystem.EXPECT().Getwd().Return("/home/dev/project", nil)
				mockFilesystem.EXPECT().DirExists("/home/dev/project/api").Return(true)
				mockProfiles.EXPECT().Resolve(model.ProfileDefault).Return(tiers, nil)
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock"})
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock"})
			},
			config:           model.Config{Path: "fuku.yaml", Project: project, Topology: topology},
			expectedSections: []string{"Environment", "Configuration", "Services", "Topology", "Runtime"},
			expectedTally:    model.Tally{OK: 9, Idle: 6},
			expectedFile:     model.SeverityOK,
		},
		{
			name: "invalid config",
			before: func() {
				mockEnvironment.EXPECT().Getenv("SHELL").Return("/bin/zsh")
				mockEnvironment.EXPECT().Getenv("LANG").Return("en_US.UTF-8")
				mockEnvironment.EXPECT().Executable().Return("/usr/local/bin/fuku", nil)
				mockEnvironment.EXPECT().PathExecutable().Return("/usr/local/bin/fuku", nil)
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock"})
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock"})
			},
			config:           model.Config{Path: "fuku.yaml", Error: contracts.ErrInvalidConfig},
			expectedSections: []string{"Environment", "Configuration", "Services", "Topology", "Runtime"},
			expectedTally:    model.Tally{OK: 5, Idle: 9, Fail: 1},
			expectedFile:     model.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			report := NewRunner(options, tt.config, mockEnvironment, mockFilesystem, mockProfiles, mockRuntime).Run(t.Context())

			require.NotNil(t, report)
			assert.Equal(t, 1, report.SchemaVersion)
			assert.Equal(t, "0.99.0", report.Version)
			assert.NotEmpty(t, report.Platform)
			assert.False(t, report.GeneratedAt.IsZero())

			titles := make([]string, 0, len(report.Sections))
			for _, section := range report.Sections {
				titles = append(titles, section.Title)
			}

			ids := make([]model.CheckID, 0, len(rows))

			for _, section := range report.Sections {
				for _, result := range section.Results {
					ids = append(ids, result.ID)
				}
			}

			assert.Equal(t, tt.expectedSections, titles)
			assert.Equal(t, rows, ids)
			assert.Equal(t, tt.expectedTally, report.Tally())
			assert.Equal(t, tt.expectedFile, report.Sections[1].Results[0].Severity)
		})
	}
}

func Test_Runner_load(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfiles(ctrl)

	project := model.Project{
		Services: []model.Service{{Name: "web"}, {Name: "api"}},
		Profiles: map[string]model.Profile{model.ProfileDefault: {All: true}},
	}

	web := &model.Service{Name: "web"}
	api := &model.Service{Name: "api"}
	tiers := []model.Tier{{Name: model.TierDefault, Services: []*model.Service{web, api}}}

	options := Options{Profile: model.ProfileDefault, ExplicitConfig: true, Fingerprint: "0123456789abcdef"}

	tests := []struct {
		name               string
		before             func()
		config             model.Config
		expectedServices   []*model.Service
		expectedProfileErr error
	}{
		{
			name: "loaded config resolves the profile sorted by name",
			before: func() {
				mockProfiles.EXPECT().Resolve(model.ProfileDefault).Return(tiers, nil)
			},
			config:           model.Config{Path: "fuku.yaml", OverridePath: "fuku.override.yaml", Project: project},
			expectedServices: []*model.Service{api, web},
		},
		{
			name: "unknown profile keeps the resolve error",
			before: func() {
				mockProfiles.EXPECT().Resolve(model.ProfileDefault).Return(nil, contracts.ErrProfileNotFound)
			},
			config:             model.Config{Path: "fuku.yaml", Project: model.Project{}},
			expectedProfileErr: contracts.ErrProfileNotFound,
		},
		{
			name:   "failed load resolves nothing",
			before: func() {},
			config: model.Config{Path: "fuku.yaml", Error: assert.AnError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			st := NewRunner(options, tt.config, nil, nil, mockProfiles, nil).load()

			require.ErrorIs(t, st.profileErr, tt.expectedProfileErr)
			assert.Equal(t, options, st.Options)
			assert.Equal(t, tt.config, st.Config)
			assert.Equal(t, tt.expectedServices, st.services)
		})
	}
}
