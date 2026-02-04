package source

import (
	"fmt"

	"git-remote-diff/internal/gitutil"
	"git-remote-diff/internal/yamlutil"

	yaml "gopkg.in/yaml.v3"
)

// Spec represents either a local file or a Git-backed file.
type Spec struct {
	LocalPath string
	Git       *GitSpec
}

// GitSpec configures fetching a file from a repository.
type GitSpec struct {
	RepoURL  string
	Ref      gitutil.Ref
	FilePath string
}

// Loader knows how to resolve a Spec into a YAML node.
type Loader struct {
	GitAuth gitutil.Auth
}

func (l Loader) Load(spec Spec) (*yaml.Node, error) {
	switch {
	case spec.Git != nil:
		b, err := gitutil.FetchFile(spec.Git.RepoURL, spec.Git.Ref, spec.Git.FilePath, l.GitAuth)
		if err != nil {
			return nil, fmt.Errorf("fetch git file: %w", err)
		}
		n, err := yamlutil.UnmarshalYAMLBytes(b)
		if err != nil {
			return nil, fmt.Errorf("decode git yaml: %w", err)
		}
		return n, nil
	case spec.LocalPath != "":
		n, err := yamlutil.LoadYAML(spec.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("load local yaml: %w", err)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("no source configured")
	}
}
