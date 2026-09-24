package readiness

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewChecker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	assert.NotNil(t, checker)
	assert.Equal(t, mockPublisher, checker.publisher)
	assert.Equal(t, log, checker.log)
}

func Test_Checker_Check(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockProcess := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	done := make(chan struct{})

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stderr.Close()

	completed := func(readinessType model.ReadinessType) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.ReadinessComplete)

			return msg.Type == contracts.EventReadinessComplete && ok && reflect.DeepEqual(data.Service, svc) && data.Type == readinessType
		})
	}

	tests := []struct {
		name     string
		before   func(t *testing.T) model.Readiness
		expected error
	}{
		{
			name: "HTTP readiness passes and publishes the duration",
			before: func(t *testing.T) model.Readiness {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(server.Close)

				mockProcess.EXPECT().Service().Return(svc)
				mockProcess.EXPECT().Done().Return(done)
				mockPublisher.EXPECT().Publish(completed(model.ReadinessHTTP)).Return(nil)

				return model.Readiness{Type: model.ReadinessHTTP, URL: server.URL, Timeout: 5 * time.Second, Interval: 100 * time.Millisecond}
			},
		},
		{
			name: "TCP readiness passes and publishes the duration",
			before: func(t *testing.T) model.Readiness {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				t.Cleanup(func() { listener.Close() })

				address := listener.Addr().String()

				mockProcess.EXPECT().Service().Return(svc)
				mockProcess.EXPECT().Done().Return(done)
				mockPublisher.EXPECT().Publish(completed(model.ReadinessTCP)).Return(nil)

				return model.Readiness{Type: model.ReadinessTCP, Address: address, Timeout: 5 * time.Second, Interval: 100 * time.Millisecond}
			},
		},
		{
			name: "log readiness reads the process streams",
			before: func(_ *testing.T) model.Readiness {
				go func() {
					stdoutWriter.Write([]byte("Server ready on port 8080\n"))
				}()

				mockProcess.EXPECT().Service().Return(svc)
				mockProcess.EXPECT().Done().Return(done)
				mockProcess.EXPECT().Stdout().Return(stdout)
				mockProcess.EXPECT().Stderr().Return(stderr)
				mockPublisher.EXPECT().Publish(completed(model.ReadinessLog)).Return(nil)

				return model.Readiness{Type: model.ReadinessLog, Pattern: "ready", Timeout: 2 * time.Second}
			},
		},
		{
			name: "a failed check returns the error and publishes nothing",
			before: func(t *testing.T) model.Readiness {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				t.Cleanup(server.Close)

				mockProcess.EXPECT().Service().Return(svc)
				mockProcess.EXPECT().Done().Return(done)

				return model.Readiness{Type: model.ReadinessHTTP, URL: server.URL, Timeout: 50 * time.Millisecond, Interval: 10 * time.Millisecond}
			},
			expected: contracts.ErrReadinessTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readiness := tt.before(t)

			err := checker.Check(t.Context(), readiness, mockProcess)

			require.ErrorIs(t, err, tt.expected)
		})
	}

	stdoutWriter.Close()
	stderrWriter.Close()
}

func Test_Checker_contextWithDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	tests := []struct {
		name   string
		before func() (context.Context, <-chan struct{})
	}{
		{
			name: "cancels when the process exits",
			before: func() (context.Context, <-chan struct{}) {
				done := make(chan struct{})
				close(done)

				return t.Context(), done
			},
		},
		{
			name: "cancels when the parent is cancelled",
			before: func() (context.Context, <-chan struct{}) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx, make(chan struct{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, done := tt.before()

			ctx, cancel := checker.contextWithDone(parent, done)
			defer cancel()

			<-ctx.Done()
			require.ErrorIs(t, ctx.Err(), context.Canceled)
		})
	}
}
