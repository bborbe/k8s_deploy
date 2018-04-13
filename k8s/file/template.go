package provider

import (
	"path"

	io_util "github.com/bborbe/io/util"
	"github.com/seibert-media/k8s-deploy/k8s"
)

// TemplateDirectory for all namespaces
type TemplateDirectory string

func (t TemplateDirectory) String() string {
	return string(t)
}

// NormalizePath to replace ~/ with absolute homedir
func (t TemplateDirectory) NormalizePath() (TemplateDirectory, error) {
	root, err := io_util.NormalizePath(t.String())
	if err != nil {
		return "", err
	}
	return TemplateDirectory(root), nil
}

// PathToNamespace the NamespaceDirectory.
func (t *TemplateDirectory) PathToNamespace(namespace k8s.Namespace) NamespaceDirectory {
	return NamespaceDirectory(path.Join(t.String(), namespace.String()))
}
