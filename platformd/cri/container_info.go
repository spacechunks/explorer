package cri

import "fmt"

type Namespace struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type Linux struct {
	CgroupsPath string      `json:"cgroupsPath"`
	Namespaces  []Namespace `json:"namespaces"`
}

type RuntimeSpec struct {
	Linux Linux `json:"linux"`
}

type ContainerInfo struct {
	RuntimeSpec RuntimeSpec `json:"runtimeSpec"`
}

type NamespaceType string

const (
	NamespaceTypePid NamespaceType = "pid"
	NamespaceTypeNet NamespaceType = "network"
)

func FindNsPath(nsType NamespaceType, namespaces []Namespace) (string, error) {
	var path string
	for _, n := range namespaces {
		if n.Type != string(nsType) {
			continue
		}
		path = n.Path
	}

	if path == "" {
		return "", fmt.Errorf("no network namespace found")
	}

	return path, nil
}
