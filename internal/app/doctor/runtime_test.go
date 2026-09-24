package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Runner_runtimeSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntime := NewMockRuntime(ctrl)
	mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock"})
	mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock"})

	subject := NewRunner(Options{}, model.Config{}, nil, nil, nil, mockRuntime)

	st := &state{Options: Options{Fingerprint: "0123456789abcdef"}, Config: model.Config{Error: assert.AnError}}

	section := subject.runtimeSection(t.Context(), st)

	assert.Equal(t, "Runtime", section.Title)
	require.Len(t, section.Results, 3)
	assert.Equal(t, model.CheckRuntimeInstance, section.Results[0].ID)
	assert.Equal(t, model.CheckRuntimeSockets, section.Results[1].ID)
	assert.Equal(t, model.CheckRuntimePorts, section.Results[2].ID)
}

func Test_Runner_checkInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntime := NewMockRuntime(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, nil, nil, mockRuntime)

	st := &state{Options: Options{Fingerprint: "0123456789abcdef"}}

	tests := []struct {
		name                string
		before              func()
		expectedSeverity    model.Severity
		expectedSummary     string
		expectedDetails     []model.Detail
		expectedRemediation string
	}{
		{
			name: "absent socket",
			before: func() {
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock"})
			},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "no other fuku running for this project",
			expectedDetails:  []model.Detail{{Key: "socket", Value: "/tmp/fuku-0123456789abcdef.sock (absent)"}},
		},
		{
			name: "live socket",
			before: func() {
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock", Present: true, Reachable: true})
			},
			expectedSeverity: model.SeverityNote,
			expectedSummary:  "another fuku is running for this project",
			expectedDetails:  []model.Detail{{Key: "socket", Value: "/tmp/fuku-0123456789abcdef.sock"}},
		},
		{
			name: "stale socket",
			before: func() {
				mockRuntime.EXPECT().Socket("0123456789abcdef").Return(model.Socket{Path: "/tmp/fuku-0123456789abcdef.sock", Present: true, Error: assert.AnError})
			},
			expectedSeverity:    model.SeverityWarn,
			expectedSummary:     "socket present but unreachable",
			expectedDetails:     []model.Detail{{Key: "socket", Value: "/tmp/fuku-0123456789abcdef.sock"}, {Key: "error", Value: assert.AnError.Error()}},
			expectedRemediation: "remove the stale socket: rm /tmp/fuku-0123456789abcdef.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkInstance(st)

			assert.Equal(t, model.CheckRuntimeInstance, r.ID)
			assert.Equal(t, tt.expectedSeverity, r.Severity)
			assert.Equal(t, tt.expectedSummary, r.Summary)
			assert.Equal(t, tt.expectedDetails, r.Details)
			assert.Equal(t, tt.expectedRemediation, r.Remediation)
		})
	}
}

func Test_Runner_checkStaleSockets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntime := NewMockRuntime(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, nil, nil, mockRuntime)

	tests := []struct {
		name                string
		before              func()
		expectedSeverity    model.Severity
		expectedSummary     string
		expectedDetails     []model.Detail
		expectedRemediation string
	}{
		{
			name: "no sockets",
			before: func() {
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock"})
			},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "no stale sockets",
			expectedDetails:  []model.Detail{{Key: "scanned", Value: "/tmp/fuku-*.sock (0 files)"}},
		},
		{
			name: "only live sockets",
			before: func() {
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock", Files: 2, Sockets: []model.Socket{
					{Path: "/tmp/fuku-aaaa.sock", Present: true, Reachable: true},
				}})
			},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "no stale sockets",
			expectedDetails:  []model.Detail{{Key: "scanned", Value: "/tmp/fuku-*.sock (2 files)"}},
		},
		{
			name: "stale sockets",
			before: func() {
				mockRuntime.EXPECT().Sockets().Return(model.SocketScan{Dir: "/tmp", Pattern: "/tmp/fuku-*.sock", Files: 3, Sockets: []model.Socket{
					{Path: "/tmp/fuku-aaaa.sock", Present: true, Reachable: true},
					{Path: "/tmp/fuku-bbbb.sock", Present: true, Error: assert.AnError},
					{Path: "/tmp/fuku-cccc.sock", Present: true, Error: assert.AnError},
				}})
			},
			expectedSeverity:    model.SeverityWarn,
			expectedSummary:     "2 stale socket file(s)",
			expectedDetails:     []model.Detail{{Key: "fuku-bbbb.sock", Value: "stale"}, {Key: "fuku-cccc.sock", Value: "stale"}},
			expectedRemediation: "remove the stale sockets from /tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkStaleSockets()

			assert.Equal(t, model.CheckRuntimeSockets, r.ID)
			assert.Equal(t, tt.expectedSeverity, r.Severity)
			assert.Equal(t, tt.expectedSummary, r.Summary)
			assert.Equal(t, tt.expectedDetails, r.Details)
			assert.Equal(t, tt.expectedRemediation, r.Remediation)
		})
	}
}

func Test_Runner_checkPorts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntime := NewMockRuntime(ctrl)

	subject := NewRunner(Options{}, model.Config{}, nil, nil, nil, mockRuntime)

	http := model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}
	tcp := model.Readiness{Type: model.ReadinessTCP, Address: "localhost:5432"}
	log := model.Readiness{Type: model.ReadinessLog, Pattern: "ready"}

	project := model.Project{
		Services: []model.Service{
			{Name: "api", Directory: "api"},
			{Name: "http", Readiness: &http},
			{Name: "tcp", Readiness: &tcp},
			{Name: "log", Readiness: &log},
		},
	}

	tests := []struct {
		name             string
		before           func()
		state            *state
		expectedSeverity model.Severity
		expectedSummary  string
		expectedDetails  []model.Detail
	}{
		{
			name:             "config did not load",
			before:           func() {},
			state:            &state{Config: model.Config{Error: assert.AnError}},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "skipped (config did not load)",
		},
		{
			name:             "profile did not resolve",
			before:           func() {},
			state:            &state{profileErr: assert.AnError},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "skipped (profile did not resolve)",
		},
		{
			name:             "no readiness probes",
			before:           func() {},
			state:            &state{Config: model.Config{Project: project}, services: []string{"api"}},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "no probed readiness ports",
		},
		{
			name: "log probe has no port",
			before: func() {
				mockRuntime.EXPECT().ProbePort(gomock.Any(), log).Return(model.Port{})
			},
			state:            &state{Config: model.Config{Project: project}, services: []string{"log"}},
			expectedSeverity: model.SeverityIdle,
			expectedSummary:  "no probed readiness ports",
		},
		{
			name: "ports available",
			before: func() {
				mockRuntime.EXPECT().ProbePort(gomock.Any(), http).Return(model.Port{Address: "localhost:8080"})
				mockRuntime.EXPECT().ProbePort(gomock.Any(), tcp).Return(model.Port{Address: "localhost:5432"})
			},
			state:            &state{Config: model.Config{Project: project}, services: []string{"http", "tcp"}},
			expectedSeverity: model.SeverityOK,
			expectedSummary:  "2 readiness port(s) available",
		},
		{
			name: "port already bound",
			before: func() {
				mockRuntime.EXPECT().ProbePort(gomock.Any(), http).Return(model.Port{Address: "localhost:8080", InUse: true})
				mockRuntime.EXPECT().ProbePort(gomock.Any(), tcp).Return(model.Port{Address: "localhost:5432"})
			},
			state:            &state{Config: model.Config{Project: project}, services: []string{"http", "tcp"}},
			expectedSeverity: model.SeverityWarn,
			expectedSummary:  "1 readiness port(s) already bound",
			expectedDetails:  []model.Detail{{Key: "http", Value: "localhost:8080 already LISTENING"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := subject.checkPorts(t.Context(), tt.state)

			assert.Equal(t, model.CheckRuntimePorts, r.ID)
			assert.Equal(t, tt.expectedSeverity, r.Severity)
			assert.Equal(t, tt.expectedSummary, r.Summary)
			assert.Equal(t, tt.expectedDetails, r.Details)
		})
	}
}
