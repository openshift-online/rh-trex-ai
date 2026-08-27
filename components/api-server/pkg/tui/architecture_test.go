package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresentationPolicyHasOneOwner(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]string)
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		sources[filepath.Base(path)] = string(data)
	}
	if failures := presentationPolicyViolations(sources); len(failures) > 0 {
		t.Fatalf("presentation policy duplication:\n%s", strings.Join(failures, "\n"))
	}
}

func TestArchitectureGateRejectsSyntheticPageOwnedStyle(t *testing.T) {
	failures := presentationPolicyViolations(map[string]string{"bad_page.go": `package tui
import "github.com/charmbracelet/lipgloss"
func badPage() string { return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("bad") }
`})
	if len(failures) == 0 {
		t.Fatal("architecture gate accepted page-owned presentation policy")
	}
}

func TestArchitectureGateRejectsSyntheticPageOwnedShortcutLayout(t *testing.T) {
	failures := presentationPolicyViolations(map[string]string{"bad_page.go": `package tui
func badPage(shortcuts []ShortcutHint) { LayoutShortcutPalette(shortcuts, 80, 6) }
`})
	if len(failures) == 0 {
		t.Fatal("architecture gate accepted page-owned shortcut-palette layout")
	}
}

func TestArchitectureGateRejectsSyntheticPageOwnedBreadcrumbLayout(t *testing.T) {
	failures := presentationPolicyViolations(map[string]string{"bad_page.go": `package tui
func badPage(segments []BreadcrumbSegment, theme Theme) { RenderBreadcrumb(segments, theme, 80) }
`})
	if len(failures) == 0 {
		t.Fatal("architecture gate accepted page-owned breadcrumb layout")
	}
}

func TestArchitectureGateRejectsSyntheticPageOwnedPromptLayout(t *testing.T) {
	failures := presentationPolicyViolations(map[string]string{"bad_page.go": `package tui
func badPage(theme Theme, view CommandPromptView) { theme.CommandPrompt(view, 80, 3) }
`})
	if len(failures) == 0 {
		t.Fatal("architecture gate accepted page-owned command/filter prompt layout")
	}
}

func TestArchitectureGateRejectsSyntheticPageOwnedFormAndDialogActionLayout(t *testing.T) {
	failures := presentationPolicyViolations(map[string]string{"bad_page.go": `package tui
func badPage(theme Theme, hint ShortcutHint, field formFieldDescriptor) {
	_ = formFieldType(field)
	_ = theme.DialogAction(hint, true)
	_ = theme.DialogButton("Delete", true)
}
`})
	if len(failures) < 3 {
		t.Fatalf("architecture gate accepted duplicated form/dialog action policy: %v", failures)
	}
}

func TestResourceCatalogUsesSharedTableAndPageContracts(t *testing.T) {
	data, err := os.ReadFile("catalog.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{"model.rebuildTable(view)", "model.setRows(view, model.catalogItems())"} {
		if !strings.Contains(source, required) {
			t.Fatalf("catalog does not use shared resource-table contract %q", required)
		}
	}
	for _, duplicated := range []string{"table.New(", "sort.SliceStable("} {
		if strings.Contains(source, duplicated) {
			t.Fatalf("catalog duplicates shared table policy %q", duplicated)
		}
	}
}

func presentationPolicyViolations(sources map[string]string) []string {
	var failures []string
	for name, source := range sources {
		if name != "theme.go" && (strings.Contains(source, "lipgloss.Color(") || strings.Contains(source, "lipgloss.NewStyle(")) {
			failures = append(failures, name+": raw style outside theme.go")
		}
		if name != "keys.go" && (strings.Contains(source, "key.NewBinding(") || strings.Contains(source, "key.WithKeys(")) {
			failures = append(failures, name+": key binding outside keys.go")
		}
		if name != "modal.go" && strings.Contains(source, "overlayBlock(") {
			failures = append(failures, name+": dialog positioning outside modal.go")
		}
		if name != "column_layout.go" && strings.Contains(source, "tableColumnMinimumWidth") {
			failures = append(failures, name+": column sizing policy outside column_layout.go")
		}
		if name != "shortcut_palette.go" && name != "layout.go" && strings.Contains(source, "LayoutShortcutPalette(") {
			failures = append(failures, name+": shortcut-palette layout outside shortcut_palette.go")
		}
		if name != "breadcrumb.go" && name != "shell.go" && strings.Contains(source, "RenderBreadcrumb(") {
			failures = append(failures, name+": breadcrumb layout outside breadcrumb.go")
		}
		if name != "theme.go" && name != "shell.go" && strings.Contains(source, ".CommandPrompt(") {
			failures = append(failures, name+": command/filter prompt layout outside theme.go")
		}
		if name != "form.go" && strings.Contains(source, "formFieldType(") {
			failures = append(failures, name+": form-column layout outside form.go")
		}
		if name != "theme.go" && name != "form.go" && strings.Contains(source, ".DialogAction(") {
			failures = append(failures, name+": dialog-action layout outside form.go")
		}
		if name != "theme.go" && name != "form.go" && strings.Contains(source, ".DialogButton(") {
			failures = append(failures, name+": dialog-button layout outside form.go")
		}
		if name != "alert.go" && (strings.Contains(source, "alertLifetime") || strings.Contains(source, "alertPriority(")) {
			failures = append(failures, name+": alert lifetime/priority outside alert.go")
		}
		if strings.HasSuffix(name, "page.go") && strings.Contains(source, "RoundedBorder") {
			failures = append(failures, name+": page owns outer chrome")
		}
	}
	return failures
}
