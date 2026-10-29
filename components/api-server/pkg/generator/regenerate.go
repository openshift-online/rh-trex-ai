package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Regenerate strips the manifest entities' artifacts (OpenAPI $ref entries and
// plugin imports) from the repository, then regenerates each declared entity
// from the templates. It is idempotent: running it repeatedly converges to the
// same state instead of duplicating refs or imports.
func Regenerate(opts Options, m *Manifest, repoRoot string) error {
	if len(m.Entities) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(m.Entities))
	for _, e := range m.Entities {
		dirs = append(dirs, ComputeNaming(e.Kind, e.Plural).KindLowerPlural)
	}
	sort.Strings(dirs)

	openapiPath := filepath.Join(repoRoot, "openapi", "openapi.yaml")
	if data, err := os.ReadFile(openapiPath); err == nil {
		cleaned := stripOpenAPIEntityRefs(data, dirs)
		if err := os.WriteFile(openapiPath, cleaned, 0o666); err != nil {
			return fmt.Errorf("rewriting openapi.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading openapi.yaml: %w", err)
	}

	mainPath := filepath.Join(repoRoot, "cmd", getCmdDir(repoRoot), "main.go")
	if data, err := os.ReadFile(mainPath); err == nil {
		cleaned := stripPluginImports(data, dirs)
		if err := os.WriteFile(mainPath, cleaned, 0o666); err != nil {
			return fmt.Errorf("rewriting main.go: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading main.go: %w", err)
	}

	for _, e := range m.Entities {
		entityOpts := opts
		entityOpts.BaseDir = repoRoot
		entityOpts.Kind = e.Kind
		entityOpts.Fields = e.Fields
		entityOpts.Plural = e.Plural
		entityOpts.ProtoPackage = e.ProtoPackage
		if entityOpts.ProtoPackage == "" {
			entityOpts.ProtoPackage = m.ProtoPackage
		}
		if entityOpts.ProtoPackage == "" {
			entityOpts.ProtoPackage = resolveProtoPackage(opts.Project)
		}
		entityOpts.RunMake = false
		if err := Run(entityOpts); err != nil {
			return fmt.Errorf("regenerating %s: %w", e.Kind, err)
		}
	}
	return nil
}
