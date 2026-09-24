package process

import (
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_Preflight_publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockReporter := NewMockReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	preflight := &Preflight{publisher: mockPublisher, reporter: mockReporter, log: log}

	msg := contracts.Message{Type: contracts.EventPreflightComplete, Data: contracts.PreflightComplete{}}

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "an accepted publish reports nothing",
			before: func() {
				mockPublisher.EXPECT().Publish(msg).Return(nil)
			},
		},
		{
			name: "a rejected publish is a runtime failure",
			before: func() {
				mockPublisher.EXPECT().Publish(msg).Return(contracts.ErrBusOverloaded)
				mockReporter.EXPECT().Fail(contracts.ErrBusOverloaded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			preflight.publish(msg)
		})
	}
}
