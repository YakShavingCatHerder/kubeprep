package curriculum

import (
	"fmt"
	"strings"
)

// NamespaceDocument is the Namespace object the runner applies for a lab
// declared with `namespace:`.
func NamespaceDocument(name string) []byte {
	return []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + name + "\n")
}

// ComposeResources builds the YAML the runner applies or resets. A declared
// lab namespace is always first so namespaced manifests can land in it.
func (s *Scenario) ComposeResources(resources ResourceSet, read func(string) ([]byte, error)) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("compose scenario resources: scenario is nil")
	}
	var documents [][]byte
	if name := strings.TrimSpace(s.Namespace); name != "" {
		documents = append(documents, NamespaceDocument(name))
	}
	for _, reference := range resources.Manifests {
		if read == nil {
			return nil, fmt.Errorf("read scenario manifest %q: reader is nil", reference)
		}
		document, err := read(reference)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	if len(documents) == 0 {
		return nil, nil
	}
	parts := make([]string, len(documents))
	for i, document := range documents {
		parts[i] = string(document)
	}
	return []byte(strings.Join(parts, "\n---\n")), nil
}
