package github

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

func Test_newHTTPClient(t *testing.T) {
	c := newHTTPClient()

	assert.Equal(t, DefaultTimeout, c.Timeout)
}

func Test_Client_Latest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDoer := NewMockHTTPDoer(ctrl)

	log := slog.New(slog.DiscardHandler)

	cachePath := filepath.Join(t.TempDir(), "version.json")

	checkRequest := func(req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, DefaultEndpoint, req.URL.String())
		assert.Equal(t, "application/vnd.github+json", req.Header.Get("Accept"))
		assert.Equal(t, "fuku/"+buildinfo.Version, req.Header.Get("User-Agent"))
	}

	tests := []struct {
		name             string
		before           func()
		options          Options
		expected         model.Release
		expectedErr      error
		expectedCacheTag string
	}{
		{
			name: "fresh cache is served without a request",
			before: func() {
				require.NoError(t, writeCache(cachePath, cache{Tag: "v0.99.0", FetchedAt: time.Now().Add(-time.Hour)}))
			},
			options:          Options{CachePath: cachePath},
			expected:         model.Release{Tag: "v0.99.0"},
			expectedCacheTag: "v0.99.0",
		},
		{
			name: "stale cache fetches and rewrites the cache",
			before: func() {
				require.NoError(t, writeCache(cachePath, cache{Tag: "v0.05.0", FetchedAt: time.Now().Add(-48 * time.Hour)}))
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.23.0"}`))}, nil)
			},
			options:          Options{CachePath: cachePath},
			expected:         model.Release{Tag: "v0.23.0"},
			expectedCacheTag: "v0.23.0",
		},
		{
			name: "unreadable cache fetches and rewrites the cache",
			before: func() {
				require.NoError(t, os.WriteFile(cachePath, []byte("{not json"), 0o600))
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.24.0"}`))}, nil)
			},
			options:          Options{CachePath: cachePath},
			expected:         model.Release{Tag: "v0.24.0"},
			expectedCacheTag: "v0.24.0",
		},
		{
			name: "no cache fetches and writes the cache",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.22.0"}`))}, nil)
			},
			options:          Options{CachePath: cachePath},
			expected:         model.Release{Tag: "v0.22.0"},
			expectedCacheTag: "v0.22.0",
		},
		{
			name: "tag without v prefix is returned and cached as sent",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"0.30.0"}`))}, nil)
			},
			options:          Options{CachePath: cachePath},
			expected:         model.Release{Tag: "0.30.0"},
			expectedCacheTag: "0.30.0",
		},
		{
			name: "cache path unavailable still fetches",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.40.0"}`))}, nil)
			},
			options:  Options{},
			expected: model.Release{Tag: "v0.40.0"},
		},
		{
			name: "request carries the GitHub headers and the endpoint",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Do(checkRequest).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.20.0"}`))}, nil)
			},
			options:  Options{},
			expected: model.Release{Tag: "v0.20.0"},
		},
		{
			name: "network error",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(nil, assert.AnError)
			},
			options:     Options{CachePath: cachePath},
			expectedErr: assert.AnError,
		},
		{
			name: "not found status",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil)
			},
			options:     Options{CachePath: cachePath},
			expectedErr: ErrUnexpectedReleaseStatus,
		},
		{
			name: "server error status",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{}`))}, nil)
			},
			options:     Options{CachePath: cachePath},
			expectedErr: ErrUnexpectedReleaseStatus,
		},
		{
			name: "empty tag",
			before: func() {
				mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":""}`))}, nil)
			},
			options:     Options{CachePath: cachePath},
			expectedErr: ErrEmptyReleaseTag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.RemoveAll(cachePath))
			tt.before()

			got, err := NewClient(tt.options, mockDoer, log).Latest(t.Context())

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expected, got)

			entry, err := readCache(cachePath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCacheTag, entry.Tag)
		})
	}
}

func Test_Client_Latest_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDoer := NewMockHTTPDoer(ctrl)
	mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`not json`))}, nil)

	log := slog.New(slog.DiscardHandler)

	cachePath := filepath.Join(t.TempDir(), "version.json")

	got, err := NewClient(Options{CachePath: cachePath}, mockDoer, log).Latest(t.Context())

	require.Error(t, err)
	assert.Equal(t, model.Release{}, got)
	assert.NoFileExists(t, cachePath)
}

func Test_Client_Latest_UnwritableCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDoer := NewMockHTTPDoer(ctrl)
	mockDoer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.50.0"}`))}, nil)

	mockLogger := NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug("Write cache failed", "error", gomock.Any())

	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o500))

	cachePath := filepath.Join(dir, "version.json")

	subject := NewClient(Options{CachePath: cachePath}, mockDoer, mockLogger)

	got, err := subject.Latest(t.Context())

	require.NoError(t, err)
	assert.Equal(t, model.Release{Tag: "v0.50.0"}, got)
	assert.NoFileExists(t, cachePath)
}
