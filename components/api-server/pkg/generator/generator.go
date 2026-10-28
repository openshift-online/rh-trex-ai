package generator

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Options controls a single entity generation run.
type Options struct {
	// TemplatesFS is the filesystem from which generate-*.txt templates are read.
	TemplatesFS fs.FS
	// BaseDir is the directory that generated files are written into and where
	// make commands run. Empty means the current working directory.
	BaseDir string
	Kind    string
	Repo    string
	Project string
	// Library is the Go module path of the rh-trex-ai library; framework
	// imports (pkg/api, pkg/server, plugins/events, ...) use it.
	Library string
	// Module is the Go module path of the project the entities are generated
	// into. Project-local imports (pkg/api/openapi, pkg/api/grpc, test,
	// plugins/<kind>) use it. Empty means the same as Library.
	Module string
	// ProtoPackage defaults to the snake_case form of Project when empty.
	ProtoPackage string
	// Plural overrides the auto-derived plural form when set.
	Plural string
	Fields string
	// RunMake runs `make proto` and `make generate` after generating files.
	RunMake bool
	// Quiet suppresses informational progress messages.
	Quiet bool
}

// TemplateData is the data passed to every generate-*.txt template.
type TemplateData struct {
	Repo                string
	Project             string
	ProjectPascalCase   string
	ApiProject          string
	Library             string
	Module              string
	ProtoPackage        string
	Cmd                 string
	Kind                string
	KindPlural          string
	KindLowerPlural     string
	KindLowerSingular   string
	KindSnakeCasePlural string
	ID                  string
	Fields              []Field
}

// generateTemplates lists the template names in generation order.
var generateTemplates = []string{
	"api",
	"presenters",
	"dao",
	"services",
	"mock",
	"migration",
	"test",
	"test-factories",
	"testmain",
	"handlers",
	"openapi-kind",
	"grpc-handler",
	"grpc-presenter",
	"grpc-test",
	"proto",
	"plugin",
}

// outputPathTemplates maps a template name to the output path format string.
var outputPathTemplates = map[string]string{
	"api":            "plugins/%s/model.go",
	"presenters":     "plugins/%s/presenter.go",
	"dao":            "plugins/%s/dao.go",
	"services":       "plugins/%s/service.go",
	"mock":           "plugins/%s/mock_dao.go",
	"migration":      "plugins/%s/migration.go",
	"test":           "plugins/%s/integration_test.go",
	"test-factories": "plugins/%s/factory_test.go",
	"testmain":       "plugins/%s/testmain_test.go",
	"handlers":       "plugins/%s/handler.go",
	"openapi-kind":   "openapi/openapi.%s.yaml",
	"grpc-handler":   "plugins/%s/grpc_handler.go",
	"grpc-presenter": "plugins/%s/grpc_presenter.go",
	"grpc-test":      "plugins/%s/grpc_integration_test.go",
	"proto":          "proto/%s/v1/%s.proto",
	"plugin":         "plugins/%s/plugin.go",
}

// TemplateFuncs returns the function map used by every entity template.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"protoFieldType": protoFieldType,
		"add":            func(a, b int) int { return a + b },
		"article":        article,
	}
}

// Run generates a new entity from the embedded templates.
func Run(opts Options) error {
	parsedFields, err := parseFields(opts.Fields)
	if err != nil {
		return fmt.Errorf("parsing fields: %w", err)
	}

	for _, nm := range generateTemplates {
		tmplContents, err := fs.ReadFile(opts.TemplatesFS, fmt.Sprintf("generate-%s.txt", nm))
		if err != nil {
			return fmt.Errorf("reading template generate-%s.txt: %w", nm, err)
		}

		kindTmpl, err := template.New(nm).Funcs(TemplateFuncs()).Parse(string(tmplContents))
		if err != nil {
			return fmt.Errorf("parsing template generate-%s.txt: %w", nm, err)
		}

		kindLowerSingular := strings.ToLower(string(opts.Kind[0])) + opts.Kind[1:]
		kindPlural := pluralize(opts.Kind, opts.Plural)
		kindPluralLower := pluralize(kindLowerSingular, "")
		kindPluralSnake := toSnakeCase(kindPlural)

		protoPackage := opts.ProtoPackage
		if protoPackage == "" {
			protoPackage = strings.ReplaceAll(strings.ReplaceAll(opts.Project, "-", "_"), ".", "_")
		}

		module := opts.Module
		if module == "" {
			module = opts.Library
		}

		data := TemplateData{
			Project:             opts.Project,
			ProjectPascalCase:   toPascalCase(opts.Project),
			ApiProject:          opts.Project,
			Repo:                opts.Repo,
			Library:             opts.Library,
			Module:              module,
			ProtoPackage:        protoPackage,
			Cmd:                 getCmdDir(opts.BaseDir),
			Kind:                opts.Kind,
			KindPlural:          kindPlural,
			KindLowerPlural:     kindPluralLower,
			KindLowerSingular:   kindLowerSingular,
			KindSnakeCasePlural: kindPluralSnake,
			Fields:              parsedFields,
		}
		data.ID = newID(opts.Kind)

		var outputPath string
		if nm == "proto" {
			outputPath = fmt.Sprintf(outputPathTemplates[nm], data.ProtoPackage, data.KindSnakeCasePlural)
		} else {
			outputPath = fmt.Sprintf(outputPathTemplates[nm], data.KindLowerPlural)
		}
		outputPath = filepath.Join(opts.BaseDir, outputPath)

		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}

		f, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		w := bufio.NewWriter(f)
		if err := kindTmpl.Execute(w, data); err != nil {
			return fmt.Errorf("executing template generate-%s.txt: %w", nm, err)
		}
		if err := w.Flush(); err != nil {
			return err
		}

		runGofmt(outputPath)

		if nm == "openapi-kind" {
			mainPath := filepath.Join(opts.BaseDir, "openapi", "openapi.yaml")
			if err := modifyOpenapi(mainPath, outputPath); err != nil {
				return err
			}
		}

		if nm == "plugin" {
			if err := addPluginImport(opts, data); err != nil {
				return err
			}
		}
	}

	if opts.RunMake {
		runMake(opts.BaseDir, "proto")
		runMake(opts.BaseDir, "generate")
	}

	return nil
}

func newID(kind string) string {
	now := time.Now()
	return fmt.Sprintf("%d%s%s%s%s%04d", now.Year(), datePad(int(now.Month())), datePad(now.Day()), datePad(now.Hour()), datePad(now.Minute()), kindNameHash(kind))
}

func kindNameHash(kind string) int {
	h := sha256.Sum256([]byte(kind))
	return int(binary.BigEndian.Uint16(h[:2])) % 10000
}

func datePad(d int) string {
	if d < 10 {
		return fmt.Sprintf("0%d", d)
	}
	return fmt.Sprintf("%d", d)
}

func protoFieldType(field Field) string {
	switch field.Type {
	case "string":
		return "string"
	case "int":
		return "int32"
	case "int64":
		return "int64"
	case "bool":
		return "bool"
	case "float":
		return "double"
	case "time":
		return "google.protobuf.Timestamp"
	default:
		return "string"
	}
}

func runGofmt(path string) {
	if filepath.Ext(path) != ".go" {
		return
	}
	if err := exec.Command("gofmt", "-w", path).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: gofmt failed for %s: %v\n", path, err)
	}
}

func runMake(baseDir, target string) {
	fmt.Printf("Running make %s to regenerate generated code...\n", target)
	cmd := exec.Command("make", target)
	cmd.Dir = baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: make %s failed: %v\n", target, err)
	}
}

// RunMakeSteps runs `make proto` then `make generate` in baseDir after
// entity files have been written. It is used by callers that want the
// derived protobuf/OpenAPI client artifacts regenerated once per batch.
func RunMakeSteps(baseDir string) {
	runMake(baseDir, "proto")
	runMake(baseDir, "generate")
}
