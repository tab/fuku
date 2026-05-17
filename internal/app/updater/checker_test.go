package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewChecker(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := &config.Config{Updater: true}
	mockBus := bus.NewMockBus(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	c := NewChecker(cfg, mockBus, mockLog)

	assert.NotNil(t, c)
}

func Test_Checker_Run(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockBus := bus.NewMockBus(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	subject := &checker{
		bus:        mockBus,
		log:        mockLog,
		httpClient: &http.Client{Timeout: httpTimeout},
	}

	tests := []struct {
		name              string
		before            func()
		updaterEnabled    bool
		cacheTag          string
		cacheAge          time.Duration
		cachePathFails    bool
		httpStatus        int
		httpBody          string
		forceNetworkError bool
		expectedCacheTag  string
	}{
		{
			name:           "disabled - no HTTP, no publish",
			before:         func() {},
			updaterEnabled: false,
		},
		{
			name: "cache fresh and newer - publishes from cache",
			before: func() {
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.EventUpdateAvailable, msg.Type)
					data, ok := msg.Data.(bus.UpdateAvailable)
					assert.True(t, ok)
					assert.Equal(t, "v0.99.0", data.Version)
				})
			},
			updaterEnabled:   true,
			cacheTag:         "v0.99.0",
			cacheAge:         time.Hour,
			expectedCacheTag: "v0.99.0",
		},
		{
			name:             "cache fresh and same - no publish",
			before:           func() {},
			updaterEnabled:   true,
			cacheTag:         "v" + config.Version,
			cacheAge:         time.Hour,
			expectedCacheTag: "v" + config.Version,
		},
		{
			name:             "cache fresh and older - no publish",
			before:           func() {},
			updaterEnabled:   true,
			cacheTag:         "v0.10.0",
			cacheAge:         time.Hour,
			expectedCacheTag: "v0.10.0",
		},
		{
			name: "cache stale and HTTP newer - publishes and writes cache",
			before: func() {
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.EventUpdateAvailable, msg.Type)
					data, ok := msg.Data.(bus.UpdateAvailable)
					assert.True(t, ok)
					assert.Equal(t, "v0.21.0", data.Version)
				})
			},
			updaterEnabled:   true,
			cacheTag:         "v0.05.0",
			cacheAge:         48 * time.Hour,
			httpStatus:       http.StatusOK,
			httpBody:         `{"tag_name":"v0.21.0"}`,
			expectedCacheTag: "v0.21.0",
		},
		{
			name: "no cache and HTTP newer - publishes and writes cache",
			before: func() {
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.EventUpdateAvailable, msg.Type)
					data, ok := msg.Data.(bus.UpdateAvailable)
					assert.True(t, ok)
					assert.Equal(t, "v0.22.0", data.Version)
				})
			},
			updaterEnabled:   true,
			httpStatus:       http.StatusOK,
			httpBody:         `{"tag_name":"v0.22.0"}`,
			expectedCacheTag: "v0.22.0",
		},
		{
			name:             "no cache and HTTP same version - no publish but writes cache",
			before:           func() {},
			updaterEnabled:   true,
			httpStatus:       http.StatusOK,
			httpBody:         `{"tag_name":"v` + config.Version + `"}`,
			expectedCacheTag: "v" + config.Version,
		},
		{
			name:              "network error - no publish, no cache write",
			before:            func() {},
			updaterEnabled:    true,
			forceNetworkError: true,
		},
		{
			name:           "HTTP 404 - no publish, no cache write",
			before:         func() {},
			updaterEnabled: true,
			httpStatus:     http.StatusNotFound,
			httpBody:       `{}`,
		},
		{
			name:           "HTTP 500 - no publish, no cache write",
			before:         func() {},
			updaterEnabled: true,
			httpStatus:     http.StatusInternalServerError,
			httpBody:       `{}`,
		},
		{
			name:           "invalid JSON - no publish, no cache write",
			before:         func() {},
			updaterEnabled: true,
			httpStatus:     http.StatusOK,
			httpBody:       `not json`,
		},
		{
			name:           "empty tag in response - no publish, no cache write",
			before:         func() {},
			updaterEnabled: true,
			httpStatus:     http.StatusOK,
			httpBody:       `{"tag_name":""}`,
		},
		{
			name:           "invalid semver tag - no publish, no cache write",
			before:         func() {},
			updaterEnabled: true,
			httpStatus:     http.StatusOK,
			httpBody:       `{"tag_name":"not-a-version"}`,
		},
		{
			name: "tag without v prefix - normalized and published",
			before: func() {
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.EventUpdateAvailable, msg.Type)
					data, ok := msg.Data.(bus.UpdateAvailable)
					assert.True(t, ok)
					assert.Equal(t, "v0.30.0", data.Version)
				})
			},
			updaterEnabled:   true,
			httpStatus:       http.StatusOK,
			httpBody:         `{"tag_name":"0.30.0"}`,
			expectedCacheTag: "0.30.0",
		},
		{
			name: "cache path unavailable - still fetches and publishes",
			before: func() {
				mockBus.EXPECT().Publish(gomock.Any()).Do(func(msg bus.Message) {
					assert.Equal(t, bus.EventUpdateAvailable, msg.Type)
					data, ok := msg.Data.(bus.UpdateAvailable)
					assert.True(t, ok)
					assert.Equal(t, "v0.40.0", data.Version)
				})
			},
			updaterEnabled: true,
			cachePathFails: true,
			httpStatus:     http.StatusOK,
			httpBody:       `{"tag_name":"v0.40.0"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "version.json")

			origCachePathFn := cachePathFn

			switch {
			case tt.cachePathFails:
				cachePathFn = func() (string, error) { return "", assert.AnError }
			default:
				cachePathFn = func() (string, error) { return path, nil }
			}

			t.Cleanup(func() { cachePathFn = origCachePathFn })

			if tt.cacheTag != "" {
				require.NoError(t, writeCache(path, cache{
					Tag:       tt.cacheTag,
					FetchedAt: time.Now().Add(-tt.cacheAge),
				}))
			}

			origReleaseURL := releaseURL

			t.Cleanup(func() { releaseURL = origReleaseURL })

			switch {
			case tt.forceNetworkError:
				releaseURL = "http://127.0.0.1:1/closed"
			case tt.httpStatus != 0:
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.httpStatus)
					_, _ = w.Write([]byte(tt.httpBody))
				}))
				t.Cleanup(server.Close)
				releaseURL = server.URL
			}

			subject.cfg = &config.Config{Updater: tt.updaterEnabled}

			tt.before()

			subject.Run(context.Background())

			entry, err := readCache(path)
			require.NoError(t, err)

			switch {
			case tt.expectedCacheTag != "":
				assert.Equal(t, tt.expectedCacheTag, entry.Tag, "cache tag mismatch")
			default:
				assert.Empty(t, entry.Tag, "expected no cache to be present")
			}
		})
	}
}

func Test_Checker_FetchLatest_SetsRequiredHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)

	var capturedAccept, capturedUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		capturedUserAgent = r.Header.Get("User-Agent")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v0.20.0"}`))
	}))
	t.Cleanup(server.Close)

	origReleaseURL := releaseURL
	releaseURL = server.URL

	t.Cleanup(func() { releaseURL = origReleaseURL })

	c := &checker{
		bus:        bus.NewMockBus(ctrl),
		cfg:        &config.Config{Updater: true},
		log:        logger.NewMockLogger(ctrl),
		httpClient: &http.Client{Timeout: httpTimeout},
	}

	tag, err := c.fetchLatest(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "v0.20.0", tag)
	assert.Equal(t, "application/vnd.github+json", capturedAccept)
	assert.Equal(t, "fuku/"+config.Version, capturedUserAgent)
}

func Test_Checker_FetchLatest_BadURL(t *testing.T) {
	ctrl := gomock.NewController(t)

	origReleaseURL := releaseURL
	releaseURL = "://malformed"

	t.Cleanup(func() { releaseURL = origReleaseURL })

	c := &checker{
		bus:        bus.NewMockBus(ctrl),
		cfg:        &config.Config{Updater: true},
		log:        logger.NewMockLogger(ctrl),
		httpClient: &http.Client{Timeout: httpTimeout},
	}

	_, err := c.fetchLatest(context.Background())

	assert.Error(t, err)
}
