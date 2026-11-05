package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
)

// EntitySpec describes one entity to generate.
type EntitySpec struct {
	Kind         string `json:"kind" yaml:"kind"`
	Fields       string `json:"fields" yaml:"fields"`
	Plural       string `json:"plural,omitempty" yaml:"plural,omitempty"`
	ProtoPackage string `json:"proto_package,omitempty" yaml:"proto_package,omitempty"`
}

// Manifest is the declarative description of all auto-generated entities.
type Manifest struct {
	ProtoPackage string       `json:"proto_package,omitempty" yaml:"proto_package,omitempty"`
	Entities     []EntitySpec `json:"entities" yaml:"entities"`
	// Ignore lists relative paths that are allowed to deviate from the
	// regenerated output (e.g. hand-tuned golden files). Entries may be
	// filepath.Match patterns.
	Ignore []string `json:"ignore,omitempty" yaml:"ignore,omitempty"`
}

// ignores reports whether rel is an allowed deviation.
func (m *Manifest) ignores(rel string) bool {
	for _, pat := range m.Ignore {
		if pat == rel {
			return true
		}
		if ok, err := filepath.Match(pat, rel); err == nil && ok {
			return true
		}
	}
	return false
}

// EntityOptions returns the options to generate entity e of manifest m, based
// on the command-line options opts. The protobuf package is, in order: the
// entity's proto_package, the --proto-package flag, the manifest's
// proto_package, then the snake_case project name.
func EntityOptions(opts Options, m *Manifest, e EntitySpec) Options {
	entityOpts := opts
	entityOpts.Kind = e.Kind
	entityOpts.Fields = e.Fields
	entityOpts.Plural = e.Plural
	entityOpts.ProtoPackage = e.ProtoPackage
	if entityOpts.ProtoPackage == "" {
		entityOpts.ProtoPackage = opts.ProtoPackage
	}
	if entityOpts.ProtoPackage == "" {
		entityOpts.ProtoPackage = m.ProtoPackage
	}
	if entityOpts.ProtoPackage == "" {
		entityOpts.ProtoPackage = resolveProtoPackage(opts.Project)
	}
	entityOpts.RunMake = false
	return entityOpts
}

// LoadManifest reads a YAML manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}

	// An empty entities list is valid (e.g. a fresh downstream project that
	// has not declared entities yet); the verifier treats it as no drift.
	for _, e := range m.Entities {
		if e.Kind == "" {
			return nil, fmt.Errorf("manifest %s contains an entity without a kind", path)
		}
		if err := ValidateKind(e.Kind, e.Plural); err != nil {
			return nil, fmt.Errorf("manifest %s: %w", path, err)
		}
	}

	return &m, nil
}
