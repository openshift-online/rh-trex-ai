package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/templates"
)

const (
	placeholderProject      = "my-service"
	placeholderProjectSnake = "my_service"
	placeholderRepo         = "github.com/example"
)

// NewProjectCommand scaffolds a new TRex-based application from the embedded
// new-project template. Run it as:
//
//	trex new-project my-service --repo github.com/myorg
//
// The resulting project is a complete Go service that imports the TRex
// library and inherits all commands (serve, migrate, generate-entity, ...).
func NewProjectCommand() *cobra.Command {
	var repo string
	var dir string

	cmd := &cobra.Command{
		Use:   "new-project <name>",
		Short: "Scaffold a new TRex-based application",
		Long:  "Scaffold a new TRex-based application from the embedded template. The name must be a valid Go package / binary name (e.g. my-service).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := validateProjectName(name); err != nil {
				return err
			}
			return scaffoldProject(name, repo, dir)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&repo, "repo", placeholderRepo, "module base path for the new project (e.g. github.com/myorg)")
	flags.StringVar(&dir, "dir", "", "directory to create the project in (default: current directory)")

	return cmd
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name must not be empty")
	}
	if strings.ContainsAny(name, " /\\") {
		return fmt.Errorf("project name %q must not contain spaces or path separators", name)
	}
	return nil
}

func scaffoldProject(name, repo, dir string) error {
	target := filepath.Join(dir, name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("target directory %s already exists", target)
	}

	snake := toSnake(name)
	replacer := strings.NewReplacer(
		placeholderProject, name,
		placeholderProjectSnake, snake,
		placeholderRepo, repo,
	)

	if err := fs.WalkDir(templates.NewProjectFS, "new-project", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("new-project", path)
		if err != nil {
			return err
		}
		rel = replacer.Replace(rel)
		// The template's go.mod is stored as go.mod.tmpl because an embedded
		// go.mod would make it a nested module, which go:embed forbids.
		if rel == "go.mod.tmpl" {
			rel = "go.mod"
		}
		dest := filepath.Join(target, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(templates.NewProjectFS, path)
		if err != nil {
			return err
		}
		content := replacer.Replace(string(data))
		// Strip the "//go:build ignore" header added so the embedded template
		// is not compiled as part of the TRex module.
		if strings.HasSuffix(rel, ".go") {
			content = strings.TrimPrefix(content, "//go:build ignore\n\n")
		}
		return os.WriteFile(dest, []byte(content), 0o644)
	}); err != nil {
		return err
	}

	fmt.Printf("Created new TRex project at %s\n", target)
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", target)
	fmt.Printf("  make generate   # generate OpenAPI client\n")
	fmt.Printf("  go mod tidy     # resolve dependencies\n")
	fmt.Printf("  make binary      # build %s\n", name)
	fmt.Printf("  ./%s new-entity --kind HelloWorld --fields \"message:string:required\" --project %s --repo %s\n", name, name, repo)
	return nil
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
