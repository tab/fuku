package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_LogView_Status_WritesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)

	var buf bytes.Buffer

	status := contracts.LogStatus{Profile: "default", Version: "1.0.0", Services: []string{"api", "web"}}

	NewLogView(mockFormatter, &buf).Status(status, []string{"api"})

	assert.Empty(t, buf.String())
}

func Test_LogView_Line(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFormatter := NewMockFormatter(ctrl)
	mockFormatter.EXPECT().FormatMessage("api", "request processed").Return("api | request processed\n")

	var buf bytes.Buffer

	NewLogView(mockFormatter, &buf).Line(model.LogLine{Service: "api", Message: "request processed"})

	assert.Equal(t, "api | request processed\n", buf.String())
}
