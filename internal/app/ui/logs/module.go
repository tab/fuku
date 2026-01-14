package logs

import (
	"go.uber.org/fx"

	"fuku/internal/app/ui"
)

// Module provides the logs UI components
var Module = fx.Options(
	fx.Provide(
		NewLogView,
		NewSender,
		NewSubscriber,
	),
)

// NewLogView creates a new LogView implementation
func NewLogView(filter ui.LogFilter) ui.LogView {
	model := NewModel(filter)

	return &model
}
