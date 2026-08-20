package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type treeEntry struct {
	Path   string
	Mode   fs.FileMode
	Digest [sha256.Size]byte
}

func TestDeterministicGeneratedDescriptorPackage(t *testing.T) {
	first := filepath.Join(t.TempDir(), "generated")
	second := filepath.Join(t.TempDir(), "generated")
	options := generateOptions{SpecPath: "testdata/navigation.yaml"}
	options.OutDir = first
	if err := generate(options); err != nil {
		t.Fatal(err)
	}
	options.OutDir = second
	if err := generate(options); err != nil {
		t.Fatal(err)
	}
	firstTree := snapshotTree(t, first)
	secondTree := snapshotTree(t, second)
	if !reflect.DeepEqual(firstTree, secondTree) {
		t.Fatalf("generated trees differ:\nfirst:  %#v\nsecond: %#v", firstTree, secondTree)
	}
	wantPaths := []string{outputMarker, "descriptor.go", "descriptor.json"}
	gotPaths := make([]string, 0, len(firstTree))
	for _, entry := range firstTree {
		gotPaths = append(gotPaths, entry.Path)
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("generated descriptor package paths = %v, want %v", gotPaths, wantPaths)
	}

	worktree, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range firstTree {
		data, err := os.ReadFile(filepath.Join(first, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(worktree)) {
			t.Fatalf("generated %s contains host path %s", entry.Path, worktree)
		}
		if strings.HasSuffix(entry.Path, ".go") && !bytes.HasPrefix(data, []byte("// Code generated")) {
			t.Fatalf("generated Go file %s lacks a stable notice", entry.Path)
		}
	}
}

func TestRepositoryOpenAPIGeneratesIntegratedServiceCommand(t *testing.T) {
	host := t.TempDir()
	output := filepath.Join(host, "data", "generated", "tui")
	if err := generate(generateOptions{
		SpecPath: filepath.Join("..", "..", "openapi", "openapi.yaml"),
		OutDir:   output,
	}); err != nil {
		t.Fatal(err)
	}
	descriptor, err := os.ReadFile(filepath.Join(output, "descriptor.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, operationID := range []string{
		"listDinosaurs", "createDinosaur", "getDinosaur", "updateDinosaur", "deleteDinosaur",
		"listFossils", "createFossil", "getFossil", "updateFossil", "deleteFossil",
		"listScientists", "createScientist", "getScientist", "updateScientist", "deleteScientist",
	} {
		if !bytes.Contains(descriptor, []byte(`"`+operationID+`"`)) {
			t.Fatalf("repository descriptor omitted operation %s", operationID)
		}
	}
	writeIntegratedHarness(t, host)
	runGeneratedCommand(t, host, "go", "build", "-mod=mod", "-o", "service", "./cmd/service")
	help := runGeneratedCommand(t, host, filepath.Join(host, "service"), "--help")
	if !strings.Contains(help, "tui") || !strings.Contains(help, "terminal UI") {
		t.Fatalf("integrated service help omitted TUI command:\n%s", help)
	}
	tuiHelp := runGeneratedCommand(t, host, filepath.Join(host, "service"), "tui", "--help")
	for _, flag := range []string{"--server", "--token-file", "--insecure", "--trust-origin", "--refresh-interval"} {
		if !strings.Contains(tuiHelp, flag) {
			t.Fatalf("integrated TUI help omitted %s:\n%s", flag, tuiHelp)
		}
	}
}

func TestGenerationReplacesOutputWithoutTouchingSibling(t *testing.T) {
	parent := t.TempDir()
	output := filepath.Join(parent, "generated")
	sibling := output + ".previous"
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "stale"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, outputMarker), []byte(outputMarkerContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := generate(generateOptions{
		SpecPath: "testdata/navigation.yaml", OutDir: output,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale output remains after replacement: %v", err)
	}
	content, err := os.ReadFile(sibling)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "unrelated" {
		t.Fatalf("sibling changed during output replacement: %q", content)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".tui-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary backups remain after generation: %v", matches)
	}
}

func TestGenerationRefusesToReplaceUnownedOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "source-repository")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(output, "keep-me")
	if err := os.WriteFile(sentinel, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := generate(generateOptions{
		SpecPath: "testdata/navigation.yaml", OutDir: output,
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace unowned output") {
		t.Fatalf("generation error = %v, want unowned-output refusal", err)
	}
	content, readErr := os.ReadFile(sentinel)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "unrelated" {
		t.Fatalf("unowned output changed: %q", content)
	}
}

func TestGenerationRefusesSymbolicLinkOutput(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "owned-target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep-me")
	if err := os.WriteFile(sentinel, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(parent, "generated")
	if err := os.Symlink(target, output); err != nil {
		t.Fatal(err)
	}
	err := generate(generateOptions{
		SpecPath: "testdata/navigation.yaml", OutDir: output,
	})
	if err == nil || !strings.Contains(err.Error(), "symbolic-link output") {
		t.Fatalf("generation error = %v, want symbolic-link refusal", err)
	}
	if content, readErr := os.ReadFile(sentinel); readErr != nil || string(content) != "unrelated" {
		t.Fatalf("symbolic-link target changed: content=%q err=%v", content, readErr)
	}
	if info, statErr := os.Lstat(output); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("output symlink changed: info=%v err=%v", info, statErr)
	}
}

func snapshotTree(t *testing.T, root string) []treeEntry {
	t.Helper()
	var entries []treeEntry
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, treeEntry{Path: filepath.ToSlash(relative), Mode: info.Mode(), Digest: sha256.Sum256(data)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func runGeneratedCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v in %s: %v\n%s", name, arguments, directory, err, output)
	}
	return string(output)
}

func writeIntegratedHarness(t *testing.T, root string) {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	module := fmt.Sprintf(`module example.com/integrated-service

go 1.24.2

require github.com/openshift-online/rh-trex-ai v0.0.0

replace github.com/openshift-online/rh-trex-ai => %s
`, filepath.ToSlash(repository))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "cmd", "service", "main.go")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0o755); err != nil {
		t.Fatal(err)
	}
	mainSource := `package main

import (
	"fmt"
	"os"

	generatedtui "example.com/integrated-service/data/generated/tui"
	pkgcmd "github.com/openshift-online/rh-trex-ai/pkg/cmd"
)

func main() {
	root := pkgcmd.NewRootCommand("service", "Integrated service")
	root.AddCommand(pkgcmd.NewTUICommand(generatedtui.GetDescriptor))
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(mainPath, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}
}
