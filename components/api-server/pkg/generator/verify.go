package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Naming holds every computed name variant for a kind.
type Naming struct {
	Kind                string
	KindPlural          string
	KindLowerSingular   string
	KindLowerPlural     string
	KindSnakeCasePlural string
}

// ComputeNaming derives all naming variants for a kind.
func ComputeNaming(kind, plural string) Naming {
	kindLowerSingular := strings.ToLower(string(kind[0])) + kind[1:]
	kindPlural := pluralize(kind, plural)
	return Naming{
		Kind:                kind,
		KindPlural:          kindPlural,
		KindLowerSingular:   kindLowerSingular,
		KindLowerPlural:     pluralize(kindLowerSingular, ""),
		KindSnakeCasePlural: toSnakeCase(kindPlural),
	}
}

// entityOutputPaths returns the relative paths the generator writes for an
// entity, excluding migration.go (its ID is timestamp-based by design).
func entityOutputPaths(n Naming, protoPackage string) []string {
	var paths []string
	for name, format := range outputPathTemplates {
		if name == "migration" {
			continue
		}
		if name == "proto" {
			paths = append(paths, fmt.Sprintf(format, protoPackage, n.KindSnakeCasePlural))
			continue
		}
		paths = append(paths, fmt.Sprintf(format, n.KindLowerPlural))
	}
	return paths
}

// Verify regenerates every manifest entity into a temporary directory and
// compares the per-entity artifacts, the aggregate openapi.yaml, and the
// application main.go against the files on disk in repoRoot. It returns a
// list of drift descriptions; an empty list means everything matches.
// Migration files are excluded (their IDs embed a timestamp), as are derived
// artifacts (proto stubs, OpenAPI client) produced by make proto / make
// generate. Non-manifest entities that exist on disk (for example leftover
// example entities) are stripped from both sides before the aggregate and
// main.go are compared, so they do not cause false drift.
func Verify(opts Options, m *Manifest, repoRoot string) ([]string, error) {
	if len(m.Entities) == 0 {
		return nil, fmt.Errorf("manifest declares no entities")
	}

	baseDir, err := os.MkdirTemp("", "trex-verify-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(baseDir) }()

	// Seed with baselines stripped of ALL entity content (manifest and
	// non-manifest), keeping the core OpenAPI spec and the library plugin
	// imports. Re-running generation then re-adds exactly the manifest
	// entities, in deterministic order.
	if err := seedBaseDir(repoRoot, baseDir, m); err != nil {
		return nil, err
	}

	for _, e := range m.Entities {
		entityOpts := opts
		entityOpts.BaseDir = baseDir
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
		entityOpts.Quiet = true

		if err := Run(entityOpts); err != nil {
			return nil, fmt.Errorf("generating %s in temp dir: %w", e.Kind, err)
		}
	}

	var drift []string
	for _, e := range m.Entities {
		protoPackage := e.ProtoPackage
		if protoPackage == "" {
			protoPackage = m.ProtoPackage
		}
		if protoPackage == "" {
			protoPackage = resolveProtoPackage(opts.Project)
		}
		n := ComputeNaming(e.Kind, e.Plural)
		for _, rel := range entityOutputPaths(n, protoPackage) {
			if m.ignores(rel) {
				continue
			}
			p, err := os.ReadFile(filepath.Join(baseDir, rel))
			if err != nil {
				return nil, fmt.Errorf("reading generated %s: %w", rel, err)
			}
			f, err := os.ReadFile(filepath.Join(repoRoot, rel))
			if err != nil {
				drift = append(drift, fmt.Sprintf("%s: missing in repo (expected by generator)", rel))
				continue
			}
			if !bytesEqual(p, f) {
				drift = append(drift, fmt.Sprintf("%s: differs from regenerated output", rel))
			}
		}
	}

	// Compare aggregate openapi.yaml and main.go, with non-manifest entity
	// content stripped from the on-disk copies.
	nonManifest := nonManifestEntities(repoRoot, m)
	drift = append(drift, compareProjected(
		filepath.Join(baseDir, "openapi", "openapi.yaml"),
		filepath.Join(repoRoot, "openapi", "openapi.yaml"),
		func(b []byte) []byte { return stripOpenAPIEntityRefs(b, nonManifest) },
		"openapi/openapi.yaml",
	)...)
	drift = append(drift, compareProjected(
		filepath.Join(baseDir, "cmd", getCmdDir(baseDir), "main.go"),
		filepath.Join(repoRoot, "cmd", getCmdDir(repoRoot), "main.go"),
		func(b []byte) []byte { return stripPluginImports(b, nonManifest) },
		"cmd/"+getCmdDir(repoRoot)+"/main.go",
	)...)

	return drift, nil
}

// compareProjected compares two files after applying the same projection to
// both sides, so content that is intentionally absent from generation (for
// example leftover entities) is ignored.
func compareProjected(expectedPath, actualPath string, project func([]byte) []byte, label string) []string {
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		return []string{fmt.Sprintf("%s: could not read regenerated output: %v", label, err)}
	}
	actual, err := os.ReadFile(actualPath)
	if err != nil {
		return []string{fmt.Sprintf("%s: missing in repo", label)}
	}
	if !bytesEqual(project(expected), project(actual)) {
		return []string{fmt.Sprintf("%s: differs from regenerated output", label)}
	}
	return nil
}

// nonManifestEntities returns the plural plugin directory names that exist on
// disk but are not declared in the manifest (and are not library plugins).
func nonManifestEntities(repoRoot string, m *Manifest) []string {
	manifestPlurals := make(map[string]bool)
	for _, e := range m.Entities {
		manifestPlurals[ComputeNaming(e.Kind, e.Plural).KindLowerPlural] = true
	}

	var nonManifest []string
	pluginsDir := filepath.Join(repoRoot, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nonManifest
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "events" || name == "generic" || manifestPlurals[name] {
			continue
		}
		nonManifest = append(nonManifest, name)
	}
	sort.Strings(nonManifest)
	return nonManifest
}

// stripPluginImports removes blank imports of the given plugin directories
// from a main.go file.
func stripPluginImports(mainData []byte, dirs []string) []byte {
	if len(dirs) == 0 {
		return mainData
	}
	lines := strings.Split(string(mainData), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		stripped := false
		for _, d := range dirs {
			if strings.HasPrefix(trimmed, "_ ") && strings.HasSuffix(trimmed, fmt.Sprintf("/plugins/%s\"", d)) {
				stripped = true
				break
			}
		}
		if !stripped {
			out = append(out, line)
		}
	}
	return []byte(strings.Join(out, "\n"))
}

// stripOpenAPIEntityRefs removes $ref blocks referencing the given per-kind
// OpenAPI files (for example "openapi.<kind>.yaml").
func stripOpenAPIEntityRefs(data []byte, kinds []string) []byte {
	if len(kinds) == 0 {
		return data
	}
	var targets []string
	for _, k := range kinds {
		targets = append(targets, fmt.Sprintf("openapi.%s.yaml", k))
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		ref := false
		for _, t := range targets {
			if strings.Contains(line, t) {
				ref = true
				break
			}
		}
		if ref {
			// drop the preceding key line (path key or schema name)
			if len(out) > 0 {
				prev := strings.TrimSpace(out[len(out)-1])
				if prev != "" && !strings.HasPrefix(prev, "#") {
					out = out[:len(out)-1]
				}
			}
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

func seedBaseDir(repoRoot, baseDir string, m *Manifest) error {
	cmdDir := filepath.Join(baseDir, "cmd", getCmdDir(repoRoot))
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		return err
	}

	mainSrc := filepath.Join(repoRoot, "cmd", getCmdDir(repoRoot), "main.go")
	mainData, err := os.ReadFile(mainSrc)
	if err != nil {
		return fmt.Errorf("reading %s: %w", mainSrc, err)
	}

	// Strip every plugin entity import (manifest + non-manifest), keeping
	// library plugin imports (events, generic) and the core imports. The
	// manifest entities are re-added by generation below.
	allEntities := allEntityDirs(repoRoot, m)
	mainData = stripPluginImports(mainData, allEntities)
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), mainData, 0644); err != nil {
		return err
	}

	openapiDir := filepath.Join(baseDir, "openapi")
	if err := os.MkdirAll(openapiDir, 0755); err != nil {
		return err
	}
	openapiSrc := filepath.Join(repoRoot, "openapi", "openapi.yaml")
	openapiData, err := os.ReadFile(openapiSrc)
	if err != nil {
		return fmt.Errorf("reading %s: %w", openapiSrc, err)
	}

	// Strip every per-kind $ref so generation re-adds the manifest ones.
	openapiData = stripOpenAPIEntityRefs(openapiData, allEntities)
	return os.WriteFile(filepath.Join(openapiDir, "openapi.yaml"), openapiData, 0644)
}

// allEntityDirs returns every plugin directory that is an entity plugin
// (anything other than the library plugins events/generic).
func allEntityDirs(repoRoot string, m *Manifest) []string {
	nonManifest := nonManifestEntities(repoRoot, m)
	dirs := append([]string{}, nonManifest...)
	for _, e := range m.Entities {
		dirs = append(dirs, ComputeNaming(e.Kind, e.Plural).KindLowerPlural)
	}
	sort.Strings(dirs)
	return dirs
}

func resolveProtoPackage(project string) string {
	if project == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(project, "-", "_"), ".", "_")
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
