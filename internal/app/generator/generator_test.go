package generator

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config/logger"
)

func newTestLogger(ctrl *gomock.Controller) *logger.MockLogger {
	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	noopEvent := noopLogger.Info()
	mockLog.EXPECT().Info().Return(noopEvent).AnyTimes()

	return mockLog
}

func Test_DefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, "default", opts.ProfileName)
	assert.Equal(t, "api", opts.ServiceName)
}

func Test_NewGenerator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	assert.NotNil(t, gen)
}

func Test_Generator_Generate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := DefaultOptions()

	err := gen.Generate(opts, false, false)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, fileName))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "version: 1")
	assert.Contains(t, string(content), "api:")
	assert.Contains(t, string(content), "default:")
	assert.Contains(t, string(content), "server started")
	assert.Contains(t, string(content), "x-readiness-log: &readiness-log")
	assert.Contains(t, string(content), "x-readiness-http: &readiness-http")
	assert.Contains(t, string(content), "http://localhost:8080/healthz")
}

func Test_Generator_Generate_CustomOptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := Options{ProfileName: "dev", ServiceName: "backend"}

	err := gen.Generate(opts, false, false)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, fileName))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "backend:")
	assert.Contains(t, string(content), "dev:")
}

func Test_Generator_Generate_FileExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)
	_ = os.WriteFile(fileName, []byte("existing"), 0600)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := DefaultOptions()

	err := gen.Generate(opts, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func Test_Generator_Generate_ForceOverwrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)
	_ = os.WriteFile(fileName, []byte("existing"), 0600)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := DefaultOptions()

	err := gen.Generate(opts, true, false)
	assert.NoError(t, err)

	content, err := os.ReadFile(fileName)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "version: 1")
}

func Test_Generator_Generate_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := DefaultOptions()

	err := gen.Generate(opts, false, true)
	assert.NoError(t, err)

	_, err = os.Stat(fileName)
	assert.True(t, os.IsNotExist(err))
}

func Test_Generator_Generate_DryRun_IgnoresExistingFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	defer func() { _ = os.Chdir(oldDir) }()

	_ = os.Chdir(tmpDir)
	_ = os.WriteFile(fileName, []byte("existing"), 0600)

	mockLog := newTestLogger(ctrl)
	gen := NewGenerator(mockLog)
	opts := DefaultOptions()

	err := gen.Generate(opts, false, true)
	assert.NoError(t, err)

	content, err := os.ReadFile(fileName)
	assert.NoError(t, err)
	assert.Equal(t, "existing", string(content))
}
