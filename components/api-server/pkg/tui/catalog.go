package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	resourceCatalogID    = "__trex_resource_catalog__"
	resourceCatalogLabel = "Resources"
)

func resourceCatalogView() View {
	return View{
		ID: resourceCatalogID, Kind: "collection", Label: resourceCatalogLabel,
		IdentityProperty: "view_id", DefaultSort: "resource",
		Columns:   []Column{{Property: "resource", Label: "RESOURCE", Priority: 100, Type: "string"}},
		FillWidth: true,
	}
}

func (model *Model) rebuildCatalog() {
	view := resourceCatalogView()
	model.rebuildTable(view)
	model.setRows(view, model.catalogItems())
	model.loading = false
}

func (model *Model) catalogItems() []map[string]any {
	items := make([]map[string]any, 0, len(model.descriptor.Views))
	for _, view := range model.descriptor.Views {
		if view.ListOperationID == "" || len(view.ScopeParameters) > 0 {
			continue
		}
		items = append(items, map[string]any{"view_id": view.ID, "resource": view.Label})
	}
	return items
}

func (model *Model) openCatalogSelection() (tea.Model, tea.Cmd) {
	row := model.ResourceTableComponent.Selected()
	if row == nil {
		model.alertWarning("catalog-selection", "No resource is selected")
		return model, nil
	}
	viewID := scalarString(row.Raw["view_id"])
	view := model.descriptor.View(viewID)
	if view == nil || view.ListOperationID == "" {
		model.alertError("catalog-selection", "Selected resource is unavailable")
		return model, nil
	}
	bindings := availableBindings(model.frames)
	missing := missingBindings(view.ScopeParameters, bindings)
	if len(missing) > 0 {
		model.alertWarning("catalog-context", SanitizeCell(view.Label)+" requires context: "+strings.Join(missing, ", "))
		return model, nil
	}
	return model, model.openResourceView(view, bindings, false)
}

func (model *Model) openResourceView(view *View, bindings map[string]any, resetToCatalog bool) tea.Cmd {
	if resetToCatalog {
		catalog := model.catalogFrame()
		model.frames = []Frame{catalog}
	}
	model.frames[0].CatalogSelection = view.ID
	model.frames = append(model.frames, Frame{
		ID: model.newFrameID(), TargetViewID: view.ID, Label: view.Label,
		Bindings: cloneBindings(bindings),
	})
	model.filter = ""
	model.mode = modeBrowse
	model.rebuildTable(*view)
	return model.loadCurrent()
}

func (model *Model) catalogFrame() Frame {
	if len(model.frames) > 0 && model.frames[0].Catalog {
		frame := model.frames[0]
		frame.Bindings = map[string]any{}
		frame.InFlight = false
		frame.Refreshing = false
		return frame
	}
	return Frame{ID: model.newFrameID(), Catalog: true, Label: resourceCatalogLabel, Bindings: map[string]any{}}
}

func (model *Model) commandReturnMode() mode {
	if model.previousMode == modeCatalog {
		return modeCatalog
	}
	return modeBrowse
}

func missingBindings(names []string, values map[string]any) []string {
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if _, present := values[name]; !present {
			missing = append(missing, SanitizeCell(name))
		}
	}
	sort.Strings(missing)
	return missing
}
