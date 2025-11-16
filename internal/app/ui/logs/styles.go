package logs

import (
	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

var (
	serviceNameStyle = lipgloss.NewStyle().
				Foreground(components.ColorPrimary).
				Bold(true)

	timestampStyle  = components.TimestampStyle
	emptyStateStyle = components.EmptyStateStyle

	logMessageStyle = lipgloss.NewStyle()
)
