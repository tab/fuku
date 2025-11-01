package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	assert.NotNil(t, s)
	instance, ok := s.(*service)
	assert.True(t, ok)
	assert.Equal(t, mockReadiness, instance.readiness)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Start_DirectoryNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	ctx := context.Background()
	svc := &config.Service{
		Dir: "/nonexistent/directory/path",
	}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_Start_MissingEnvFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := &config.Service{
		Dir: tmpDir,
	}

	_, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start command")
}

func Test_Start_RelativePathConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	ctx := context.Background()
	svc := &config.Service{
		Dir: "nonexistent",
	}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_Stop_NilProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	mockProcess := NewMockProcess(ctrl)
	mockCmd := &exec.Cmd{Process: nil}
	mockProcess.EXPECT().Cmd().Return(mockCmd)

	err := s.Stop(mockProcess)
	assert.NoError(t, err)
}

func Test_Start_WithValidDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")
	err := os.WriteFile(makefilePath, []byte("run:\n\techo 'test'\n"), 0644)
	require.NoError(t, err)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockReadiness := NewMockReadiness(ctrl)

	s := NewService(mockReadiness, mockLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := &config.Service{
		Dir: tmpDir,
	}

	proc, err := s.Start(ctx, "test-service", svc)

	if err != nil {
		assert.Contains(t, err.Error(), "failed to start command")
	} else {
		assert.NotNil(t, proc)
		cancel()
		if proc != nil {
			s.Stop(proc)
		}
	}
}
