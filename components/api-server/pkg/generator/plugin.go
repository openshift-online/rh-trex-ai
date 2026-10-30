package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getCmdDir(baseDir string) string {
	cmdDir := filepath.Join(baseDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return entry.Name()
		}
	}

	panic("No command directory found in cmd/")
}

// addPluginImport registers the generated plugin in cmd/<cmd>/main.go so that
// its init() runs when the application starts.
func addPluginImport(opts Options, data TemplateData) error {
	mainFile := filepath.Join(opts.BaseDir, "cmd", data.Cmd, "main.go")

	input, err := os.ReadFile(mainFile)
	if err != nil {
		return err
	}

	pluginImport := fmt.Sprintf(`_ "%s/plugins/%s"`, data.Module, data.KindLowerPlural)
	if strings.Contains(string(input), pluginImport) {
		if !opts.Quiet {
			fmt.Printf("Plugin import already exists in %s\n", mainFile)
		}
		return nil
	}

	lines := strings.Split(string(input), "\n")
	var output []string

	for i, line := range lines {
		if strings.Contains(line, "import (") {
			output = append(output, line)
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == ")" {
					output = append(output, "\t"+pluginImport)
					output = append(output, lines[j])
					output = append(output, lines[j+1:]...)
					if err := os.WriteFile(mainFile, []byte(strings.Join(output, "\n")), 0666); err != nil {
						return err
					}
					// gofmt sorts the import block, matching a
					// gofmt-checked repository.
					runGofmt(mainFile)
					if !opts.Quiet {
						fmt.Printf("Added plugin import to %s\n", mainFile)
					}
					return nil
				}
				output = append(output, lines[j])
			}
		}
		output = append(output, line)
	}

	return fmt.Errorf("could not find import block in %s", mainFile)
}
