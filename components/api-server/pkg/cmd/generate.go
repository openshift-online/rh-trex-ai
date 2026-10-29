package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/generator"
	"github.com/openshift-online/rh-trex-ai/components/api-server/templates"
)

func NewGenerateEntityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate-entity",
		Aliases: []string{"new-entity"},
		Short:   "Generate a new entity (CRUD) from embedded templates",
		Long:    "Generate a new entity (CRUD) from embedded templates. Run from the api-server source directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := generateEntityOptions(cmd)
			if err != nil {
				return err
			}
			return runGenerateEntity(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.String("kind", "Asteroid", "the name of the kind. e.g Account or User")
	flags.String("repo", "github.com/openshift-online", "the name of the repo. e.g github.com/yourproject")
	flags.String("project", "rh-trex-ai", "the name of the project. e.g rh-trex")
	flags.String("fields", "", "comma-separated list of custom fields in format name:type (e.g. 'name:string,age:int,active:bool')")
	flags.String("plural", "", "the plural form of the kind. If not provided, uses irregular plurals map or adds 's'")
	flags.String("library", "github.com/openshift-online/rh-trex-ai/components/api-server", "the module path of the rh-trex-ai library")
	flags.String("module", "", "the Go module path of the project entities are generated into (default: same as --library)")
	flags.String("proto-package", "", "the protobuf package name (e.g. rh_trex_mgr). Defaults to snake_case of project name")
	flags.String("dir", "", "directory to write generated files into (default: current directory)")
	flags.String("manifest", "", "YAML manifest describing entities to generate or verify (specs/codegen/entities.yaml)")
	flags.Bool("verify", false, "regenerate manifest entities in a temp dir and compare against disk; exit non-zero on drift")
	flags.Bool("regenerate", false, "strip and regenerate manifest entities in place (idempotent)")
	flags.String("templates-dir", "", "directory containing generate-*.txt template files (defaults to embedded templates)")
	flags.Bool("run-make", true, "run make proto and make generate after generating files")

	return cmd
}

func runGenerateEntity(cmd *cobra.Command, opts generator.Options) error {
	manifestPath, err := cmd.Flags().GetString("manifest")
	if err != nil {
		return err
	}

	if manifestPath == "" {
		return generator.Run(opts)
	}

	if !filepath.IsAbs(manifestPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		manifestPath = filepath.Join(cwd, manifestPath)
	}

	m, err := generator.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	verifyOnly, err := cmd.Flags().GetBool("verify")
	if err != nil {
		return err
	}
	if verifyOnly {
		repoRoot := opts.BaseDir
		if repoRoot == "" {
			repoRoot, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		drift, err := generator.Verify(opts, m, repoRoot)
		if err != nil {
			return err
		}
		if len(drift) > 0 {
			fmt.Fprintf(os.Stderr, "Generated entities out of sync with manifest %s:\n", manifestPath)
			for _, d := range drift {
				fmt.Fprintln(os.Stderr, "  -", d)
			}
			return fmt.Errorf("entities out of sync")
		}
		fmt.Printf("OK: all %d entities in manifest %s match generated output\n", len(m.Entities), manifestPath)
		return nil
	}

	regenerate, err := cmd.Flags().GetBool("regenerate")
	if err != nil {
		return err
	}
	if regenerate {
		repoRoot := opts.BaseDir
		if repoRoot == "" {
			repoRoot, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		if err := generator.Regenerate(opts, m, repoRoot); err != nil {
			return err
		}
		if opts.RunMake {
			generator.RunMakeSteps(repoRoot)
		}
		fmt.Printf("Regenerated %d entities from manifest %s\n", len(m.Entities), manifestPath)
		return nil
	}

	for _, e := range m.Entities {
		entityOpts := opts
		entityOpts.Kind = e.Kind
		entityOpts.Fields = e.Fields
		entityOpts.Plural = e.Plural
		entityOpts.ProtoPackage = e.ProtoPackage
		if entityOpts.ProtoPackage == "" {
			entityOpts.ProtoPackage = m.ProtoPackage
		}
		entityOpts.RunMake = false
		if err := generator.Run(entityOpts); err != nil {
			return fmt.Errorf("generating %s: %w", e.Kind, err)
		}
	}
	if opts.RunMake {
		generator.RunMakeSteps(opts.BaseDir)
	}
	return nil
}

func generateEntityOptions(cmd *cobra.Command) (generator.Options, error) {
	flags := cmd.Flags()

	kind, err := flags.GetString("kind")
	if err != nil {
		return generator.Options{}, err
	}
	repo, err := flags.GetString("repo")
	if err != nil {
		return generator.Options{}, err
	}
	project, err := flags.GetString("project")
	if err != nil {
		return generator.Options{}, err
	}
	fields, err := flags.GetString("fields")
	if err != nil {
		return generator.Options{}, err
	}
	plural, err := flags.GetString("plural")
	if err != nil {
		return generator.Options{}, err
	}
	library, err := flags.GetString("library")
	if err != nil {
		return generator.Options{}, err
	}
	module, err := flags.GetString("module")
	if err != nil {
		return generator.Options{}, err
	}
	protoPackage, err := flags.GetString("proto-package")
	if err != nil {
		return generator.Options{}, err
	}
	dir, err := flags.GetString("dir")
	if err != nil {
		return generator.Options{}, err
	}
	runMake, err := flags.GetBool("run-make")
	if err != nil {
		return generator.Options{}, err
	}
	templatesDir, err := flags.GetString("templates-dir")
	if err != nil {
		return generator.Options{}, err
	}

	templatesFS := fs.FS(templates.GeneratorFS)
	if templatesDir != "" {
		templatesFS = os.DirFS(templatesDir)
	}

	return generator.Options{
		TemplatesFS:  templatesFS,
		BaseDir:      dir,
		Kind:         kind,
		Repo:         repo,
		Project:      project,
		Library:      library,
		Module:       module,
		ProtoPackage: protoPackage,
		Plural:       plural,
		Fields:       fields,
		RunMake:      runMake,
	}, nil
}
