package mcp

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Server_findAPI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockAPI(ctrl)
	mockCandidate := NewMockAPI(ctrl)

	token := "secret-token"
	fingerprint := "this-project"
	errNotLoopback := errors.New("the fuku REST API address is not a loopback address: 192.168.1.2:3858")

	type dialed struct {
		address string
		token   string
	}

	var (
		dials   []dialed
		answers []API
		failure error
	)

	dialAPI := func(address, token string) (API, error) {
		dials = append(dials, dialed{address: address, token: token})

		if failure != nil {
			return nil, failure
		}

		answer := answers[0]
		answers = answers[1:]

		return answer, nil
	}
	dial := func(model.Instance) Client { return nil }
	load := func(string) model.Config { return model.Config{} }

	subject, err := NewServer(dial, dialAPI, load, NewBinary(), terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	tests := []struct {
		name          string
		before        func()
		status        contracts.LogStatus
		server        model.Server
		expected      API
		expectedDials []dialed
		expectedError error
	}{
		{
			name: "the address the status frame announces",
			before: func() {
				answers = []API{mockAPI}
			},
			status:        contracts.LogStatus{API: "localhost:3858"},
			server:        model.Server{Listen: "127.0.0.1:4000", Token: token},
			expected:      mockAPI,
			expectedDials: []dialed{{address: "localhost:3858", token: token}},
		},
		{
			name: "the configured address whose liveness names this project",
			before: func() {
				answers = []API{mockCandidate, mockAPI}

				mockCandidate.EXPECT().Live(gomock.Any()).Return(model.Instance{Fingerprint: fingerprint}, nil)
			},
			server:        model.Server{Listen: "127.0.0.1:4000", Token: token},
			expected:      mockAPI,
			expectedDials: []dialed{{address: "127.0.0.1:4000"}, {address: "127.0.0.1:4000", token: token}},
		},
		{
			name: "the configured address of another project",
			before: func() {
				answers = []API{mockCandidate}

				mockCandidate.EXPECT().Live(gomock.Any()).Return(model.Instance{Fingerprint: "another-project"}, nil)
			},
			server:        model.Server{Listen: "127.0.0.1:4000", Token: token},
			expectedDials: []dialed{{address: "127.0.0.1:4000"}},
			expectedError: contracts.ErrAPIUnreachable,
		},
		{
			name: "the configured address without a fuku liveness answer",
			before: func() {
				answers = []API{mockCandidate}

				mockCandidate.EXPECT().Live(gomock.Any()).Return(model.Instance{}, errors.New("the address does not serve the fuku REST API: other"))
			},
			server:        model.Server{Listen: "127.0.0.1:4000", Token: token},
			expectedDials: []dialed{{address: "127.0.0.1:4000"}},
			expectedError: contracts.ErrAPIUnreachable,
		},
		{
			name:          "the API is off",
			before:        func() {},
			server:        model.Server{Token: token},
			expectedError: contracts.ErrAPIUnreachable,
		},
		{
			name: "an announced address the client refuses",
			before: func() {
				failure = errNotLoopback
			},
			status:        contracts.LogStatus{API: "192.168.1.2:3858"},
			server:        model.Server{Token: token},
			expectedDials: []dialed{{address: "192.168.1.2:3858", token: token}},
			expectedError: errNotLoopback,
		},
		{
			name: "a configured address the client refuses",
			before: func() {
				failure = errNotLoopback
			},
			server:        model.Server{Listen: "192.168.1.2:3858", Token: token},
			expectedDials: []dialed{{address: "192.168.1.2:3858"}},
			expectedError: errNotLoopback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dials, answers, failure = nil, nil, nil

			tt.before()

			result, err := subject.findAPI(t.Context(), tt.status, tt.server, fingerprint)

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedDials, dials)
		})
	}
}
