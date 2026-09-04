package curriculum

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

var forbiddenClusterKinds = map[string]struct{}{
	"ClusterRole":                    {},
	"ClusterRoleBinding":             {},
	"CustomResourceDefinition":       {},
	"MutatingWebhookConfiguration":   {},
	"ValidatingWebhookConfiguration": {},
	"Node":                           {},
}

func validateManifestSafety(name string, data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for document := 1; ; document++ {
		var object map[string]any
		err := decoder.Decode(&object)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("manifest %q document %d: decode YAML: %w", name, document, err)
		}
		if len(object) == 0 {
			continue
		}
		kind, _ := object["kind"].(string)
		if _, forbidden := forbiddenClusterKinds[kind]; forbidden {
			return fmt.Errorf("manifest %q document %d: kind %s is not allowed in community curriculum", name, document, kind)
		}
		metadata, _ := object["metadata"].(map[string]any)
		resourceName, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)
		if kind == "Namespace" {
			if !strings.HasPrefix(resourceName, "kubecrypt-") {
				return fmt.Errorf("manifest %q document %d: namespace name must start with kubecrypt-", name, document)
			}
		} else if namespace == "" || !strings.HasPrefix(namespace, "kubecrypt-") {
			return fmt.Errorf("manifest %q document %d: resource %s/%s must use an explicit kubecrypt-* namespace", name, document, kind, resourceName)
		}
		if path, found := forbiddenManifestField(object, ""); found {
			return fmt.Errorf("manifest %q document %d: forbidden field %s", name, document, path)
		}
	}
}

func forbiddenManifestField(value any, path string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			switch key {
			case "hostPath":
				return childPath, true
			case "hostNetwork", "hostPID", "hostIPC", "privileged", "allowPrivilegeEscalation":
				if enabled, ok := child.(bool); ok && enabled {
					return childPath, true
				}
			}
			if foundPath, found := forbiddenManifestField(child, childPath); found {
				return foundPath, true
			}
		}
	case []any:
		for index, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			if foundPath, found := forbiddenManifestField(child, childPath); found {
				return foundPath, true
			}
		}
	}
	return "", false
}
