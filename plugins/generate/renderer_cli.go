package generate

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type cliResource struct {
	Name           string
	NameLower      string
	Plural         string
	PluralLower    string
	PathSegment    string
	DefaultColumns string
	WritableFields []cliField
	KindListName   string
}

type cliField struct {
	Name      string
	FlagName  string
	GoType    string
	FieldType string
}

type cliData struct {
	Binary    string
	Project   string
	APIPrefix string
	Module    string
	Resources []cliResource
	Resource  cliResource
}

func buildCLIResources(entities []ERDEntity) []cliResource {
	var resources []cliResource
	for _, e := range entities {
		fields := erdFieldsToCLIFields(e.Fields)
		pathSegment := toSnakeCase(e.Kind) + "s"
		nameLower := sdkLowerFirst(e.Kind)
		pluralName := sdkPluralize(e.Kind)
		pluralLower := sdkLowerFirst(pluralName)
		columns := buildCLIColumns(e.Fields, e.Kind)

		resources = append(resources, cliResource{
			Name:           e.Kind,
			NameLower:      nameLower,
			Plural:         pluralName,
			PluralLower:    pluralLower,
			PathSegment:    pathSegment,
			DefaultColumns: columns,
			WritableFields: fields,
			KindListName:   e.Kind + "List",
		})
	}
	return resources
}

func erdFieldsToCLIFields(fieldsStr string) []cliField {
	if fieldsStr == "" {
		return nil
	}
	var fields []cliField
	for _, pair := range strings.Split(fieldsStr, ",") {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		fieldType := parts[1]
		flagName := strings.ReplaceAll(name, "_", "-")
		goType := "string"
		switch fieldType {
		case "int", "int64":
			goType = "int"
		case "bool":
			goType = "bool"
		case "float":
			goType = "float64"
		}
		fields = append(fields, cliField{
			Name:      name,
			FlagName:  flagName,
			GoType:    goType,
			FieldType: fieldType,
		})
	}
	return fields
}

func buildCLIColumns(fieldsStr, kind string) string {
	cols := []string{"id"}
	if fieldsStr != "" {
		count := 0
		for _, pair := range strings.Split(fieldsStr, ",") {
			parts := strings.Split(strings.TrimSpace(pair), ":")
			if len(parts) >= 1 {
				cols = append(cols, parts[0])
				count++
				if count >= 4 {
					break
				}
			}
		}
	}
	cols = append(cols, "created_at")
	return strings.Join(cols, ", ")
}

var cliFuncMap = template.FuncMap{
	"lower":      strings.ToLower,
	"upper":      strings.ToUpper,
	"title":      strings.Title, //nolint:staticcheck
	"snakeCase":  toSnakeCase,
	"kebabCase":  func(s string) string { return strings.ReplaceAll(toSnakeCase(s), "_", "-") },
	"camelCase":  sdkCamelCase,
	"pascalCase": toPascalCase,
}

type cliTemplateDef struct {
	tmplFile string
	outFunc  func(cliData) string
	perRes   bool
}

var cliStaticTemplates = []cliTemplateDef{
	{"cli-cmd/main.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/main.go", d.Binary) }, false},
	{"cli-cmd/login.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/login/cmd.go", d.Binary) }, false},
	{"cli-cmd/logout.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/logout/cmd.go", d.Binary) }, false},
	{"cli-cmd/version.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/version/cmd.go", d.Binary) }, false},
	{"cli-cmd/completion.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/completion/cmd.go", d.Binary) }, false},
	{"cli-cmd/config.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/config/cmd.go", d.Binary) }, false},
	{"cli-cmd/list.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/list/cmd.go", d.Binary) }, false},
	{"cli-cmd/get.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/get/cmd.go", d.Binary) }, false},
	{"cli-cmd/create.go.tmpl", func(d cliData) string { return fmt.Sprintf("cli/cmd/%s/create/cmd.go", d.Binary) }, false},
	{"cli-pkg/config.go.tmpl", func(_ cliData) string { return "cli/pkg/config/config.go" }, false},
	{"cli-pkg/token.go.tmpl", func(_ cliData) string { return "cli/pkg/config/token.go" }, false},
	{"cli-pkg/connection.go.tmpl", func(_ cliData) string { return "cli/pkg/connection/connection.go" }, false},
	{"cli-pkg/dump.go.tmpl", func(_ cliData) string { return "cli/pkg/dump/dump.go" }, false},
	{"cli-pkg/printer.go.tmpl", func(_ cliData) string { return "cli/pkg/output/printer.go" }, false},
	{"cli-pkg/table.go.tmpl", func(_ cliData) string { return "cli/pkg/output/table.go" }, false},
	{"cli-pkg/terminal.go.tmpl", func(_ cliData) string { return "cli/pkg/output/terminal.go" }, false},
	{"cli-pkg/arguments.go.tmpl", func(_ cliData) string { return "cli/pkg/arguments/arguments.go" }, false},
	{"cli-pkg/urls.go.tmpl", func(_ cliData) string { return "cli/pkg/urls/urls.go" }, false},
	{"cli-pkg/info.go.tmpl", func(_ cliData) string { return "cli/pkg/info/info.go" }, false},
	{"cli-cmd/gomod.tmpl", func(_ cliData) string { return "cli/go.mod" }, false},
}

var cliPerResourceTemplates = []cliTemplateDef{
	{"cli-cmd/list_resource.go.tmpl", nil, true},
	{"cli-cmd/get_resource.go.tmpl", nil, true},
	{"cli-cmd/create_resource.go.tmpl", nil, true},
}

func renderCLI(entities []ERDEntity, project, apiPrefix string) ([]RenderedFile, error) {
	resources := buildCLIResources(entities)
	binary := strings.ReplaceAll(strings.ReplaceAll(project, "-", ""), "_", "")
	module := "github.com/example/" + binary + "-cli"

	data := cliData{
		Binary:    binary,
		Project:   project,
		APIPrefix: apiPrefix,
		Module:    module,
		Resources: resources,
	}

	var results []RenderedFile

	for _, td := range cliStaticTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.tmplFile)
		if err != nil {
			return nil, fmt.Errorf("reading cli template %s: %w", td.tmplFile, err)
		}
		tmpl, err := template.New(td.tmplFile).Funcs(cliFuncMap).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parsing cli template %s: %w", td.tmplFile, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("executing cli template %s: %w", td.tmplFile, err)
		}
		results = append(results, RenderedFile{
			Path:      td.outFunc(data),
			Content:   buf.String(),
			Generator: "cli",
		})
	}

	for _, td := range cliPerResourceTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.tmplFile)
		if err != nil {
			return nil, fmt.Errorf("reading cli template %s: %w", td.tmplFile, err)
		}
		for _, r := range resources {
			tmpl, err := template.New(td.tmplFile).Funcs(cliFuncMap).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("parsing cli template %s: %w", td.tmplFile, err)
			}
			rd := data
			rd.Resource = r
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, rd); err != nil {
				return nil, fmt.Errorf("executing cli template %s for %s: %w", td.tmplFile, r.Name, err)
			}
			var outPath string
			switch {
			case strings.Contains(td.tmplFile, "list_resource"):
				outPath = fmt.Sprintf("cli/cmd/%s/list/%s/cmd.go", binary, r.PluralLower)
			case strings.Contains(td.tmplFile, "get_resource"):
				outPath = fmt.Sprintf("cli/cmd/%s/get/%s/cmd.go", binary, r.NameLower)
			case strings.Contains(td.tmplFile, "create_resource"):
				outPath = fmt.Sprintf("cli/cmd/%s/create/%s/cmd.go", binary, r.NameLower)
			}
			results = append(results, RenderedFile{
				Path:      outPath,
				Content:   buf.String(),
				Generator: "cli",
			})
		}
	}

	return results, nil
}
