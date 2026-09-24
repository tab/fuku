package config

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// streamSeparator starts the override document after the base one, so the override resolves the base anchors
const streamSeparator = "\n---\n"

// mergeYAML deep-merges override YAML bytes on top of base YAML bytes and returns the merged result
func mergeYAML(base, override []byte) ([]byte, error) {
	var baseDoc yaml.Node
	if err := yaml.Unmarshal(base, &baseDoc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	overDoc, err := parseOverride(base, override)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	if overDoc.Kind != yaml.DocumentNode || len(overDoc.Content) == 0 {
		return base, nil
	}

	if baseDoc.Kind != yaml.DocumentNode || len(baseDoc.Content) == 0 {
		resolveAliases(overDoc.Content[0])

		return encodeDocument(&overDoc)
	}

	merged := mergeNodes(baseDoc.Content[0], overDoc.Content[0])
	if merged == nil {
		return base, nil
	}

	resolveAliases(merged)
	baseDoc.Content[0] = merged

	return encodeDocument(&baseDoc)
}

// encodeDocument marshals a merged document node back to YAML bytes
func encodeDocument(doc *yaml.Node) ([]byte, error) {
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
	}

	return out, nil
}

// mergeNodes recursively merges two yaml.Node trees
func mergeNodes(base, override *yaml.Node) *yaml.Node {
	base = resolveNode(base)
	override = resolveNode(override)

	if isNull(override) {
		return nil
	}

	if base.Kind != override.Kind {
		return preserveAnchor(base, override)
	}

	switch base.Kind {
	case yaml.MappingNode:
		return mergeMappings(base, override)
	case yaml.SequenceNode:
		return mergeSequences(base, override)
	default:
		return preserveAnchor(base, override)
	}
}

// mergeMappings deep-merges two mapping nodes preserving base key order
func mergeMappings(base, override *yaml.Node) *yaml.Node {
	base = flattenMergeKeys(base)
	override = flattenMergeKeys(override)

	anchor := base.Anchor
	if override.Anchor != "" {
		anchor = override.Anchor
	}

	result := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     base.Tag,
		Style:   base.Style,
		Anchor:  anchor,
		Content: make([]*yaml.Node, 0, len(base.Content)),
	}

	overrideIndex := buildKeyIndex(override)
	consumed := make(map[string]bool)

	for i := 0; i < len(base.Content); i += 2 {
		key := base.Content[i]
		baseVal := base.Content[i+1]

		idx, ok := overrideIndex[key.Value]
		if !ok {
			result.Content = append(result.Content, key, baseVal)

			continue
		}

		consumed[key.Value] = true
		overVal := override.Content[idx+1]

		if isNull(overVal) {
			continue
		}

		merged := mergeNodes(baseVal, overVal)
		if merged == nil {
			continue
		}

		result.Content = append(result.Content, key, merged)
	}

	for i := 0; i < len(override.Content); i += 2 {
		key := override.Content[i]
		val := override.Content[i+1]

		if consumed[key.Value] {
			continue
		}

		if isNull(val) {
			continue
		}

		result.Content = append(result.Content, key, flattenMergeKeys(resolveNode(val)))
	}

	return result
}

// mergeSequences concatenates base and override sequence items
func mergeSequences(base, override *yaml.Node) *yaml.Node {
	anchor := base.Anchor
	if override.Anchor != "" {
		anchor = override.Anchor
	}

	result := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     base.Tag,
		Style:   base.Style,
		Anchor:  anchor,
		Content: make([]*yaml.Node, 0, len(base.Content)+len(override.Content)),
	}

	result.Content = append(result.Content, base.Content...)
	result.Content = append(result.Content, override.Content...)

	return result
}

// isNull reports whether a node represents a YAML null value
func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// preserveAnchor returns the override node with the base anchor carried forward
func preserveAnchor(base, override *yaml.Node) *yaml.Node {
	if base.Anchor == "" || override.Anchor != "" {
		return override
	}

	clone := *override
	clone.Anchor = base.Anchor

	return &clone
}

// resolveNode dereferences an alias node to its target
func resolveNode(node *yaml.Node) *yaml.Node {
	if node.Kind != yaml.AliasNode || node.Alias == nil {
		return node
	}

	resolved := *node.Alias
	resolved.Anchor = ""

	return &resolved
}

// flattenMergeKeys expands YAML merge keys (<<) so their values participate in deep merge
func flattenMergeKeys(node *yaml.Node) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return node
	}

	hasMergeKey := false

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "<<" {
			hasMergeKey = true

			break
		}
	}

	if !hasMergeKey {
		return node
	}

	result := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     node.Tag,
		Style:   node.Style,
		Anchor:  node.Anchor,
		Content: make([]*yaml.Node, 0, len(node.Content)),
	}

	seen := make(map[string]bool)

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]

		if key.Value == "<<" {
			continue
		}

		seen[key.Value] = true

		result.Content = append(result.Content, node.Content[i], node.Content[i+1])
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]

		if key.Value != "<<" {
			continue
		}

		for _, target := range mergeKeyTargets(node.Content[i+1]) {
			target = flattenMergeKeys(target)

			for j := 0; j < len(target.Content); j += 2 {
				k := target.Content[j].Value

				if seen[k] {
					continue
				}

				seen[k] = true

				result.Content = append(result.Content, target.Content[j], target.Content[j+1])
			}
		}
	}

	return result
}

// mergeKeyTargets returns the mapping nodes referenced by a merge key value
func mergeKeyTargets(val *yaml.Node) []*yaml.Node {
	switch val.Kind {
	case yaml.AliasNode:
		if val.Alias != nil && val.Alias.Kind == yaml.MappingNode {
			return []*yaml.Node{val.Alias}
		}

		return nil
	case yaml.SequenceNode:
		var targets []*yaml.Node

		for _, item := range val.Content {
			if item.Kind == yaml.AliasNode && item.Alias != nil && item.Alias.Kind == yaml.MappingNode {
				targets = append(targets, item.Alias)
			}
		}

		return targets
	case yaml.MappingNode:
		return []*yaml.Node{val}
	default:
		return nil
	}
}

// resolveAliases replaces alias nodes with copies of their merged targets
func resolveAliases(node *yaml.Node) {
	anchors := make(map[string]*yaml.Node)
	collectAnchors(node, anchors)
	replaceAliases(node, anchors)
}

// collectAnchors builds a map of anchor names to their nodes in the merged tree
func collectAnchors(node *yaml.Node, anchors map[string]*yaml.Node) {
	if node.Anchor != "" {
		anchors[node.Anchor] = node
	}

	for _, child := range node.Content {
		collectAnchors(child, anchors)
	}
}

// replaceAliases resolves alias nodes against the merged anchors and reports false when the node should be removed
func replaceAliases(node *yaml.Node, anchors map[string]*yaml.Node) bool {
	//nolint:exhaustive // a scalar or an alias carries no children
	switch node.Kind {
	case yaml.MappingNode:
		filtered := make([]*yaml.Node, 0, len(node.Content))
		mergeKeyRemoved := false

		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			val := resolveAliasValue(node.Content[i+1], anchors)

			if val == nil {
				mergeKeyRemoved = mergeKeyRemoved || key.Value == "<<"

				continue
			}

			if !replaceAliases(val, anchors) {
				continue
			}

			filtered = append(filtered, key, val)
		}

		if mergeKeyRemoved {
			return false
		}

		node.Content = filtered
	case yaml.SequenceNode:
		filtered := make([]*yaml.Node, 0, len(node.Content))

		for _, child := range node.Content {
			resolved := resolveAliasValue(child, anchors)

			if resolved == nil {
				continue
			}

			if !replaceAliases(resolved, anchors) {
				continue
			}

			filtered = append(filtered, resolved)
		}

		node.Content = filtered
	}

	return true
}

// resolveAliasValue resolves an alias node using the merged anchor map, returning nil if the anchor was removed
func resolveAliasValue(node *yaml.Node, anchors map[string]*yaml.Node) *yaml.Node {
	if node.Kind != yaml.AliasNode || node.Alias == nil {
		return node
	}

	merged, ok := anchors[node.Alias.Anchor]
	if !ok {
		return nil
	}

	return copyNode(merged)
}

// copyNode deep-copies a yaml.Node tree, clearing anchors to avoid duplicates
func copyNode(node *yaml.Node) *yaml.Node {
	clone := *node
	clone.Anchor = ""

	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))

		for i, child := range node.Content {
			clone.Content[i] = copyNode(child)
		}
	}

	return &clone
}

// buildKeyIndex returns a map from key name to its index in a mapping node's Content slice
func buildKeyIndex(m *yaml.Node) map[string]int {
	index := make(map[string]int, len(m.Content)/2)

	for i := 0; i < len(m.Content); i += 2 {
		index[m.Content[i].Value] = i
	}

	return index
}

// parseOverride parses override YAML, decoding it after the base in one stream when it references a base anchor
func parseOverride(base, override []byte) (yaml.Node, error) {
	var overDoc yaml.Node

	err := yaml.Unmarshal(override, &overDoc)
	if err == nil {
		return overDoc, nil
	}

	stream := make([]byte, 0, len(base)+len(streamSeparator)+len(override))
	stream = append(stream, base...)
	stream = append(stream, streamSeparator...)
	stream = append(stream, override...)

	decoder := yaml.NewDecoder(bytes.NewReader(stream))

	var baseDoc yaml.Node
	if decoder.Decode(&baseDoc) != nil || decoder.Decode(&overDoc) != nil {
		return yaml.Node{}, err
	}

	return overDoc, nil
}
