package yamlutil

import (
	"errors"
	"os"

	yaml "gopkg.in/yaml.v3"
)

func LoadYAML(path string) (*yaml.Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return UnmarshalYAMLBytes(b)
}

func UnmarshalYAMLBytes(b []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		return doc.Content[0], nil
	}
	if doc.Kind == yaml.MappingNode || doc.Kind == yaml.SequenceNode {
		return &doc, nil
	}
	return nil, errors.New("unsupported YAML root kind")
}

// DiffOnlyUpdated returns a subset of `updated` that contains only fields
// that exist in `base` and have changed values in `updated`.
// - Map nodes: recurse per key present in both; include only changed children
// - Seq nodes: treated atomically; if different, include the whole updated seq
// - Scalars: include if value differs
func DiffOnlyUpdated(base, updated *yaml.Node) *yaml.Node {
	if base == nil || updated == nil {
		return nil
	}

	switch base.Kind {
	case yaml.MappingNode:
		if updated.Kind != yaml.MappingNode {
			return updated
		}
		uMap := map[string]*yaml.Node{}
		for i := 0; i < len(updated.Content)-1; i += 2 {
			key := updated.Content[i]
			val := updated.Content[i+1]
			uMap[key.Value] = val
		}
		out := &yaml.Node{Kind: yaml.MappingNode}
		for i := 0; i < len(base.Content)-1; i += 2 {
			bKey := base.Content[i]
			bVal := base.Content[i+1]
			if uVal, ok := uMap[bKey.Value]; ok {
				child := DiffOnlyUpdated(bVal, uVal)
				if child != nil && !(child.Kind == yaml.MappingNode && len(child.Content) == 0) {
					uKeyNode := findMapKeyNode(updated, bKey.Value)
					if uKeyNode == nil {
						uKeyNode = bKey
					}
					out.Content = append(out.Content, uKeyNode, child)
				}
			}
		}
		return out

	case yaml.SequenceNode:
		if !NodesEqual(base, updated) {
			return updated
		}
		return nil

	case yaml.ScalarNode:
		if !NodesEqual(base, updated) {
			return updated
		}
		return nil

	default:
		if !NodesEqual(base, updated) {
			return updated
		}
		return nil
	}
}

func findMapKeyNode(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(m.Content)-1; i += 2 {
		k := m.Content[i]
		if k.Value == key {
			return k
		}
	}
	return nil
}

func IsEmptyMapOrSeq(n *yaml.Node) bool {
	if n == nil {
		return true
	}
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		return len(n.Content) == 0
	}
	return false
}

// NodesEqual compares two nodes for semantic equality, with map key matching by string value.
func NodesEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case yaml.ScalarNode:
		return a.Tag == b.Tag && a.Value == b.Value
	case yaml.SequenceNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !NodesEqual(a.Content[i], b.Content[i]) {
				return false
			}
		}
		return true
	case yaml.MappingNode:
		bMap := map[string]*yaml.Node{}
		for i := 0; i < len(b.Content)-1; i += 2 {
			bk := b.Content[i]
			bv := b.Content[i+1]
			bMap[bk.Value] = bv
		}
		for i := 0; i < len(a.Content)-1; i += 2 {
			ak := a.Content[i]
			av := a.Content[i+1]
			bv, ok := bMap[ak.Value]
			if !ok {
				return false
			}
			if !NodesEqual(av, bv) {
				return false
			}
		}
		aMap := map[string]*yaml.Node{}
		for i := 0; i < len(a.Content)-1; i += 2 {
			ak := a.Content[i]
			av := a.Content[i+1]
			aMap[ak.Value] = av
		}
		for i := 0; i < len(b.Content)-1; i += 2 {
			bk := b.Content[i]
			bv := b.Content[i+1]
			av, ok := aMap[bk.Value]
			if !ok {
				return false
			}
			if !NodesEqual(av, bv) {
				return false
			}
		}
		return true
	case yaml.DocumentNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !NodesEqual(a.Content[i], b.Content[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
