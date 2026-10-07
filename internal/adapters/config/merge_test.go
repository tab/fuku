package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func Test_MergeYAML(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		override string
		expected string
	}{
		{
			name:     "scalar replacement",
			base:     "logging:\n  level: info\n",
			override: "logging:\n  level: debug\n",
			expected: "logging:\n  level: debug\n",
		},
		{
			name:     "nested map merge",
			base:     "services:\n  api:\n    dir: services/api\n    tier: foundation\n",
			override: "services:\n  api:\n    command: dlv debug ./cmd/main.go\n",
			expected: "services:\n  api:\n    dir: services/api\n    tier: foundation\n    command: dlv debug ./cmd/main.go\n",
		},
		{
			name:     "array concatenation without deduplication",
			base:     "services:\n  api:\n    watch:\n      include:\n        - '*.go'\n",
			override: "services:\n  api:\n    watch:\n      include:\n        - '*.templ'\n",
			expected: "services:\n  api:\n    watch:\n      include:\n        - '*.go'\n        - '*.templ'\n",
		},
		{
			name:     "null removes scalar key",
			base:     "services:\n  api:\n    dir: services/api\n    command: make run\n",
			override: "services:\n  api:\n    command: null\n",
			expected: "services:\n  api:\n    dir: services/api\n",
		},
		{
			name:     "null removes map key",
			base:     "services:\n  api:\n    dir: services/api\n    readiness:\n      type: http\n      url: http://localhost:8080\n",
			override: "services:\n  api:\n    readiness: null\n",
			expected: "services:\n  api:\n    dir: services/api\n",
		},
		{
			name:     "null removes array key",
			base:     "profiles:\n  default: \"*\"\n  backend:\n    - api\n    - auth\n",
			override: "profiles:\n  backend: null\n",
			expected: "profiles:\n  default: \"*\"\n",
		},
		{
			name:     "empty string override is kept",
			base:     "server:\n  listen: 127.0.0.1:9000\n",
			override: "server:\n  listen: \"\"\n",
			expected: "server:\n  listen: \"\"\n",
		},
		{
			name:     "mismatched kinds override wins",
			base:     "logging:\n  level: info\n  format: console\n",
			override: "logging: minimal\n",
			expected: "logging: minimal\n",
		},
		{
			name:     "override-only keys append after base keys",
			base:     "services:\n  api:\n    dir: services/api\n",
			override: "services:\n  debug-tool:\n    dir: tools/debug\n",
			expected: "services:\n  api:\n    dir: services/api\n  debug-tool:\n    dir: tools/debug\n",
		},
		{
			name:     "empty override is a no-op",
			base:     "logging:\n  level: info\n",
			override: "",
			expected: "logging:\n  level: info\n",
		},
		{
			name:     "full example from spec",
			base:     "services:\n  api:\n    dir: services/api\n    tier: foundation\n    watch:\n      include:\n        - '*.go'\nlogging:\n  level: info\n",
			override: "services:\n  api:\n    command: dlv debug ./cmd/main.go\n    watch:\n      include:\n        - '*.templ'\n  debug-tool:\n    dir: tools/debug\nlogging:\n  level: debug\n",
			expected: "services:\n  api:\n    dir: services/api\n    tier: foundation\n    watch:\n      include:\n        - '*.go'\n        - '*.templ'\n    command: dlv debug ./cmd/main.go\n  debug-tool:\n    dir: tools/debug\nlogging:\n  level: debug\n",
		},
		{
			name:     "array with duplicate values produces duplicates",
			base:     "services:\n  api:\n    watch:\n      include:\n        - '*.go'\n",
			override: "services:\n  api:\n    watch:\n      include:\n        - '*.go'\n        - '*.templ'\n",
			expected: "services:\n  api:\n    watch:\n      include:\n        - '*.go'\n        - '*.go'\n        - '*.templ'\n",
		},
		{
			name:     "exclude lists concatenate",
			base:     "exclude:\n  - api\n",
			override: "exclude:\n  - web\n  - api\n",
			expected: "exclude:\n  - api\n  - web\n  - api\n",
		},
		{
			name:     "comment-only override is a no-op",
			base:     "logging:\n  level: info\n",
			override: "# just a comment\n",
			expected: "logging:\n  level: info\n",
		},
		{
			name:     "top-level null removes key",
			base:     "services:\n  api:\n    dir: services/api\nlogging:\n  level: info\n",
			override: "logging: null\n",
			expected: "services:\n  api:\n    dir: services/api\n",
		},
		{
			name:     "top-level null override is a no-op",
			base:     "logging:\n  level: info\n",
			override: "null\n",
			expected: "logging:\n  level: info\n",
		},
		{
			name:     "empty base uses override content",
			base:     "",
			override: "logging:\n  level: debug\n",
			expected: "logging:\n  level: debug\n",
		},
		{
			name:     "comment-only base uses override content",
			base:     "# empty\n",
			override: "logging:\n  level: debug\n",
			expected: "logging:\n  level: debug\n",
		},
		{
			name:     "merge key sequence takes the first mapping's value",
			base:     "x-a: &a\n  tier: foundation\nx-b: &b\n  dir: services/api\n  tier: platform\nservices:\n  api:\n    <<: [*a, *b]\n    command: make run\n",
			override: "services:\n  api:\n    command: make debug\n",
			expected: "x-a:\n  tier: foundation\nx-b:\n  dir: services/api\n  tier: platform\nservices:\n  api:\n    command: make debug\n    tier: foundation\n    dir: services/api\n",
		},
		{
			name:     "an anchored override mapping merges into an anchored base mapping",
			base:     "x-a: &a\n  k: 1\n",
			override: "x-a: &b\n  j: 2\n",
			expected: "x-a:\n  k: 1\n  j: 2\n",
		},
		{
			name:     "an anchored override list concatenates onto an anchored base list",
			base:     "exclude: &base\n  - api\n",
			override: "exclude: &extra\n  - web\n",
			expected: "exclude:\n  - api\n  - web\n",
		},
		{
			name:     "an alias to a null removes the key",
			base:     "x-none: &none null\nlogging:\n  level: info\n",
			override: "logging: *none\n",
			expected: "x-none: null\n",
		},
		{
			name:     "null for a key the base lacks adds nothing",
			base:     "logging:\n  level: info\n",
			override: "extra: null\n",
			expected: "logging:\n  level: info\n",
		},
		{
			name:     "a merge key on an alias to a scalar merges nothing",
			base:     "x-s: &s foo\nservices:\n  api:\n    <<: *s\n    dir: a\n",
			override: "services:\n  api:\n    command: run\n",
			expected: "x-s: foo\nservices:\n  api:\n    dir: a\n    command: run\n",
		},
		{
			name:     "a merge key on a plain scalar merges nothing",
			base:     "services:\n  api:\n    <<: foo\n    dir: a\n",
			override: "services:\n  api:\n    command: run\n",
			expected: "services:\n  api:\n    dir: a\n    command: run\n",
		},
		{
			name:     "null for an anchored scalar drops its list aliases",
			base:     "x-go: &go '*.go'\nwatch:\n  - *go\n  - '*.templ'\n",
			override: "x-go: null\n",
			expected: "watch:\n  - '*.templ'\n",
		},
		{
			name:     "null for an anchored mapping drops the list items that merge it",
			base:     "x-r: &r\n  a: 1\nitems:\n  - <<: *r\n    b: 2\n  - c: 3\n",
			override: "x-r: null\n",
			expected: "items:\n  - c: 3\n",
		},
		{
			name:     "replaced anchored scalar reaches its aliases",
			base:     "x-level: &level info\nlogging:\n  level: *level\n",
			override: "x-level: debug\n",
			expected: "x-level: debug\nlogging:\n  level: debug\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mergeYAML([]byte(tt.base), []byte(tt.override))
			require.NoError(t, err)
			assertYAMLEqual(t, tt.expected, string(result))
		})
	}
}

func Test_MergeYAML_KeyOrder(t *testing.T) {
	base := "services:\n  api:\n    dir: services/api\n    tier: foundation\n"
	override := "services:\n  debug-tool:\n    dir: tools/debug\n  api:\n    command: make run\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	keys := extractMappingKeys(t, result, "services")
	assert.Equal(t, []string{"api", "debug-tool"}, keys)
}

func Test_MergeYAML_ResolvesAliases(t *testing.T) {
	base := "x-log: &log\n  type: log\n  timeout: 30s\nservices:\n  api:\n    readiness: *log\n"
	override := "x-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    watch: *watch\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	expected := "x-log:\n  type: log\n  timeout: 30s\nservices:\n  api:\n    readiness:\n      type: log\n      timeout: 30s\n    watch:\n      include:\n        - '*.go'\nx-watch:\n  include:\n    - '*.go'\n"
	assertYAMLEqual(t, expected, string(result))
}

func Test_MergeYAML_OverriddenAnchorPropagates(t *testing.T) {
	base := "x-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    watch: *watch\n"
	override := "x-watch:\n  include:\n    - '*.templ'\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	watch := api["watch"].(map[string]any)
	assert.Equal(t, []any{"*.go", "*.templ"}, watch["include"])
}

func Test_MergeYAML_AliasBackedValueDeepMerged(t *testing.T) {
	base := "x-readiness: &readiness\n  type: log\n  timeout: 30s\nservices:\n  api:\n    readiness: *readiness\n"
	override := "services:\n  api:\n    readiness:\n      timeout: 60s\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	readiness := api["readiness"].(map[string]any)
	assert.Equal(t, "log", readiness["type"])
	assert.Equal(t, "60s", readiness["timeout"])
}

func Test_MergeYAML_MergeKeyDeepMerged(t *testing.T) {
	base := "x-watch: &watch\n  include:\n    - '*.go'\n  debounce: 500ms\nservices:\n  api:\n    watch:\n      <<: *watch\n      debounce: 1s\n"
	override := "services:\n  api:\n    watch:\n      include:\n        - '*.templ'\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	watch := api["watch"].(map[string]any)
	assert.Equal(t, []any{"*.go", "*.templ"}, watch["include"])
	assert.Equal(t, "1s", watch["debounce"])
}

func Test_MergeYAML_NestedAliasResolved(t *testing.T) {
	base := "x-timeout: &timeout\n  connect: 5s\n  read: 30s\nx-readiness: &readiness\n  type: http\n  timeout: *timeout\nservices:\n  api:\n    readiness: *readiness\n"
	override := "x-timeout:\n  read: 60s\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	readiness := api["readiness"].(map[string]any)
	timeout := readiness["timeout"].(map[string]any)
	assert.Equal(t, "http", readiness["type"])
	assert.Equal(t, "5s", timeout["connect"])
	assert.Equal(t, "60s", timeout["read"])
}

func Test_MergeYAML_NullOverrideRemovesAliasReferences(t *testing.T) {
	base := "x-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    dir: services/api\n    watch: *watch\n  auth:\n    dir: services/auth\n    watch: *watch\n"
	override := "x-watch: null\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	assert.Nil(t, parsed["x-watch"])

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	assert.Equal(t, "services/api", api["dir"])
	assert.Nil(t, api["watch"])

	auth := services["auth"].(map[string]any)
	assert.Equal(t, "services/auth", auth["dir"])
	assert.Nil(t, auth["watch"])
}

func Test_MergeYAML_NullOverrideRemovesMergeKeyReferences(t *testing.T) {
	base := "x-readiness: &readiness\n  type: http\n  timeout: 30s\nservices:\n  api:\n    dir: services/api\n    readiness:\n      <<: *readiness\n      url: http://localhost:8080/health\n  auth:\n    dir: services/auth\n    readiness: *readiness\n"
	override := "x-readiness: null\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	assert.Nil(t, parsed["x-readiness"])

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	assert.Equal(t, "services/api", api["dir"])
	assert.Nil(t, api["readiness"])

	auth := services["auth"].(map[string]any)
	assert.Equal(t, "services/auth", auth["dir"])
	assert.Nil(t, auth["readiness"])
}

func Test_MergeYAML_OverrideOnlyServiceFlattensMergeKeys(t *testing.T) {
	base := "services:\n  api:\n    dir: services/api\n    tier: foundation\n"
	override := "x-svc: &svc\n  tier: foundation\nservices:\n  debug:\n    <<: *svc\n    dir: tools/debug\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	debug := services["debug"].(map[string]any)
	assert.Equal(t, "foundation", debug["tier"])
	assert.Equal(t, "tools/debug", debug["dir"])

	topology, err := parseTierOrder(result)
	require.NoError(t, err)
	assert.Contains(t, topology.TierServices["foundation"], "debug")
}

func Test_MergeYAML_OverrideReusesBaseAnchor(t *testing.T) {
	base := "x-watch: &watch\n  include:\n    - '*.go'\nservices:\n  api:\n    dir: services/api\n"
	override := "services:\n  api:\n    watch: *watch\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	api := services["api"].(map[string]any)
	assert.Equal(t, "services/api", api["dir"])

	watch := api["watch"].(map[string]any)
	assert.Equal(t, []any{"*.go"}, watch["include"])
}

func Test_MergeYAML_LayeredMergeKeyExpansion(t *testing.T) {
	base := "services:\n  api:\n    dir: services/api\n"
	override := "x-base: &base\n  tier: foundation\nx-svc: &svc\n  <<: *base\n  readiness:\n    type: http\nservices:\n  debug:\n    <<: *svc\n    dir: tools/debug\n"

	result, err := mergeYAML([]byte(base), []byte(override))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	services := parsed["services"].(map[string]any)
	debug := services["debug"].(map[string]any)
	assert.Equal(t, "foundation", debug["tier"])
	assert.Equal(t, "tools/debug", debug["dir"])

	readiness := debug["readiness"].(map[string]any)
	assert.Equal(t, "http", readiness["type"])
}

func Test_MergeYAML_InvalidBase(t *testing.T) {
	_, err := mergeYAML([]byte(":\ninvalid: [yaml"), []byte("key: value"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedToParseConfig)
}

func Test_MergeYAML_InvalidOverride(t *testing.T) {
	_, err := mergeYAML([]byte("key: value"), []byte(":\ninvalid: [yaml"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFailedToParseConfig)
}

// assertYAMLEqual compares two YAML strings semantically
func assertYAMLEqual(t *testing.T, expected, actual string) {
	t.Helper()

	var expectedVal, actualVal any
	require.NoError(t, yaml.Unmarshal([]byte(expected), &expectedVal))
	require.NoError(t, yaml.Unmarshal([]byte(actual), &actualVal))
	assert.Equal(t, expectedVal, actualVal)
}

// extractMappingKeys returns the keys of a nested mapping at the given top-level key
func extractMappingKeys(t *testing.T, data []byte, topKey string) []string {
	t.Helper()

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &root))

	doc := root.Content[0]

	for i := 0; i < len(doc.Content); i += 2 {
		if doc.Content[i].Value != topKey {
			continue
		}

		mapping := doc.Content[i+1]
		keys := make([]string, 0, len(mapping.Content)/2)

		for j := 0; j < len(mapping.Content); j += 2 {
			keys = append(keys, mapping.Content[j].Value)
		}

		return keys
	}

	t.Fatalf("key %q not found", topKey)

	return nil
}
