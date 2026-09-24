package tui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewLogView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)

	theme := terminal.NewTheme(terminal.AppearanceLight)

	var buf bytes.Buffer

	v := NewLogView(theme, mockFormatter, &buf, 80)

	require.NotNil(t, v)
	assert.Equal(t, mockFormatter, v.formatter)
	assert.Equal(t, &buf, v.out)
	assert.Equal(t, 80, v.width)
}

func Test_LogView_Status_WritesTheBanner(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceLight)

	var buf bytes.Buffer

	view := NewLogView(theme, nil, &buf, 80)

	status := contracts.LogStatus{Profile: "default", Version: "1.0.0", Services: []string{"api", "web"}}
	subscribed := []string{"api"}

	view.Status(status, subscribed)

	output := buf.String()
	assert.Contains(t, output, "default")
	assert.Contains(t, output, "2 running")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "ctrl+c")
}

func Test_LogView_Line(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)
	mockFormatter.EXPECT().FormatMessage("api", "request processed").Return("api | request processed\n")

	theme := terminal.NewTheme(terminal.AppearanceLight)

	var buf bytes.Buffer

	view := NewLogView(theme, mockFormatter, &buf, 80)

	line := model.LogLine{Service: "api", Message: "request processed"}

	view.Line(line)

	assert.Equal(t, "api | request processed\n", buf.String())
}
