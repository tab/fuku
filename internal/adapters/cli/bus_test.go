package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Announcer_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tests := []struct {
		name    string
		before  func()
		options *Options
	}{
		{
			name: "run with the TUI",
			before: func() {
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventCommandStarted,
					Data: contracts.CommandStarted{Command: "run", Profile: model.ProfileDefault, UI: true},
				}).Return(nil)
			},
			options: &Options{Type: CommandRun, Profile: model.ProfileDefault},
		},
		{
			name: "stop without a UI",
			before: func() {
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventCommandStarted,
					Data: contracts.CommandStarted{Command: "stop", Profile: "core", UI: false},
				}).Return(nil)
			},
			options: &Options{Type: CommandStop, Profile: "core", NoUI: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := NewAnnouncer(tt.options, mockPublisher).Start(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Announcer_Stop(t *testing.T) {
	subject := NewAnnouncer(&Options{}, nil)

	err := subject.Stop(t.Context())

	require.NoError(t, err)
}
