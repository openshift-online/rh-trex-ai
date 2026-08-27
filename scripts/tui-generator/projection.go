package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift-online/rh-trex-ai/pkg/tui"
	ir "github.com/openshift-online/rh-trex-ai/scripts/openapi-ir"
)

const tuiExtension = "x-trex-tui"

type projection struct {
	document     *ir.Document
	descriptor   tui.Descriptor
	views        map[string]*tui.View
	operations   map[string]*tui.Operation
	irViews      map[string]*ir.ResourceView
	irOperations map[string]*ir.Operation
	viewIDs      map[string]string
	fatal        []error
	warnings     []string
}

func projectDocument(document *ir.Document) (tui.Descriptor, error) {
	if document == nil {
		return tui.Descriptor{}, fmt.Errorf("project TUI: canonical IR document is nil")
	}
	projector := &projection{
		document:   document,
		descriptor: tui.Descriptor{Title: document.Title},
		views:      make(map[string]*tui.View), operations: make(map[string]*tui.Operation),
		irViews: make(map[string]*ir.ResourceView), irOperations: make(map[string]*ir.Operation),
		viewIDs: make(map[string]string),
	}
	for _, server := range document.Servers {
		projector.descriptor.Servers = append(projector.descriptor.Servers, tui.Server{URL: expandServer(server)})
	}
	for _, scheme := range document.SecuritySchemes {
		projector.descriptor.SecuritySchemes = append(projector.descriptor.SecuritySchemes, tui.SecurityScheme{Name: scheme.Name, Type: scheme.Type, Scheme: scheme.Scheme})
	}
	for _, operation := range document.Operations {
		projector.irOperations[operation.ID] = operation
	}
	for _, view := range document.ResourceViews {
		projector.irViews[view.ID] = view
		projector.projectView(view)
	}
	for _, operation := range document.Operations {
		projector.operation(operation)
	}
	projector.reindex()
	projector.projectOperationPresentation()
	projector.projectRelationships()
	projector.projectCollectionItemEdges()
	projector.applyExplicitPrecedence()
	projector.validateOperationHotkeyConflicts()
	projector.validateAliases()
	projector.recordUnaddressableScopes()
	projector.finish()
	projector.descriptor.Diagnostics = append(projector.descriptor.Diagnostics, projector.warnings...)
	if len(projector.fatal) > 0 {
		return projector.descriptor, errors.Join(projector.fatal...)
	}
	return projector.descriptor, nil
}

func (projector *projection) projectView(view *ir.ResourceView) {
	if view == nil {
		return
	}
	schema := projector.document.Schema(view.SchemaRef)
	if schema == nil {
		projector.fatal = append(projector.fatal, fmt.Errorf("view %s: represented schema %s is missing", view.ID, view.SchemaRef))
		return
	}
	projectedID := stableViewID(view, schema)
	projector.viewIDs[view.ID] = projectedID
	projector.irViews[projectedID] = view
	presented := tui.View{
		ID: projectedID, Kind: string(view.Kind), SchemaRef: stableSchemaRef(schema),
		ScopeParameters: append([]string(nil), view.ScopeParameters...),
		OperationIDs:    append([]string(nil), view.OperationIDs...),
	}
	sort.Strings(presented.OperationIDs)
	for _, operationID := range presented.OperationIDs {
		operation := projector.irOperations[operationID]
		if operation == nil {
			projector.fatal = append(projector.fatal, fmt.Errorf("view %s: operation %s is missing", view.ID, operationID))
			continue
		}
		projected := projector.operation(operation)
		if projected == nil {
			continue
		}
		stream := operation.Capabilities.Has(ir.CapabilityStream)
		if operation.Capabilities.Has(ir.CapabilityList) && !stream {
			presented.ListOperationID = operation.ID
		}
		if operation.Capabilities.Has(ir.CapabilityGet) && !stream {
			presented.GetOperationID = operation.ID
		}
		if stream {
			presented.StreamOperationIDs = append(presented.StreamOperationIDs, operation.ID)
		}
		for _, capability := range projected.Capabilities {
			presented.Capabilities = appendUnique(presented.Capabilities, capability)
		}
	}
	if presented.ListOperationID == "" && presented.GetOperationID == "" && len(presented.StreamOperationIDs) > 0 {
		presented.Kind = "stream"
	}
	sort.Strings(presented.StreamOperationIDs)
	sort.Strings(presented.Capabilities)
	presentationOperation := projector.irOperations[presented.ListOperationID]
	metadata, source, hasMetadata := extensionMap(presentationOperation)
	if hasMetadata && metadata == nil {
		projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "x-trex-tui must be an object"))
		hasMetadata = false
	}
	properties := projector.readableScalarProperties(schema.Ref)
	projector.applyPresentation(&presented, schema, properties, metadata, source, hasMetadata)
	projector.descriptor.Views = append(projector.descriptor.Views, presented)
	projector.reindex()
}

func (projector *projection) operation(operation *ir.Operation) *tui.Operation {
	if known := projector.operations[operation.ID]; known != nil {
		return known
	}
	projected := tui.Operation{
		ID: operation.ID, Method: operation.Method, Summary: operation.Summary,
		SuccessStatuses: successStatuses(operation), Response: projector.responseShape(operation),
		Source: tui.Source{File: operation.Source.File, Pointer: operation.Source.Pointer},
	}
	parts, err := compilePathParts(operation)
	if err != nil {
		projector.fatal = append(projector.fatal, diagnostic(operation.Source, "operation "+operation.ID, "%s", err.Error()))
		return nil
	}
	projected.PathParts = parts
	for _, parameter := range operation.Parameters {
		projected.Parameters = append(projected.Parameters, projector.parameter(parameter))
	}
	projected.RequestBody = projector.requestBody(operation)
	for _, server := range operation.Servers {
		projected.Servers = append(projected.Servers, tui.Server{URL: expandServer(server)})
	}
	projected.Security = projector.security(operation)
	stream := operation.Capabilities.Has(ir.CapabilityStream)
	for _, capability := range operation.Capabilities {
		if stream && capability == ir.CapabilityList {
			continue
		}
		projected.Capabilities = append(projected.Capabilities, string(capability))
	}
	sort.Strings(projected.Capabilities)
	projector.descriptor.Operations = append(projector.descriptor.Operations, projected)
	projector.reindex()
	return projector.operations[operation.ID]
}

func (projector *projection) applyPresentation(view *tui.View, schema *ir.Schema, properties map[string]*ir.Property, metadata map[string]any, source ir.SourceLocation, explicit bool) {
	view.Label = schema.Name
	if view.Label == "" {
		view.Label = view.ID
	}
	if property := properties["id"]; readableScalar(projector.document, property) {
		view.IdentityProperty = "id"
	}
	names := sortedPropertyNames(properties)
	for _, name := range names {
		if readableScalar(projector.document, properties[name]) {
			view.Columns = append(view.Columns, projector.presentationColumn(name, strings.ToUpper(strings.ReplaceAll(name, "_", " ")), 0, properties[name]))
		}
	}
	if len(view.Columns) > 0 {
		view.DefaultSort = view.Columns[0].Property
	}
	if !explicit {
		return
	}
	allowed := map[string]bool{"label": true, "aliases": true, "identity-property": true, "default-sort": true, "columns": true}
	for key := range metadata {
		if !allowed[key] {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "unknown x-trex-tui field %q", key))
		}
	}
	if raw, ok := metadata["label"]; ok {
		label, valid := raw.(string)
		if !valid || strings.TrimSpace(label) == "" || tui.HasTerminalControl(label) {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "label must be a non-empty terminal-safe string"))
		} else {
			view.Label = label
		}
	}
	if raw, ok := metadata["aliases"]; ok {
		aliases, err := stringSlice(raw)
		if err != nil {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "aliases: %v", err))
		} else {
			seen := make(map[string]bool)
			for _, alias := range aliases {
				if !aliasPattern.MatchString(alias) || seen[alias] {
					projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "alias %q is invalid or duplicated", alias))
					continue
				}
				seen[alias] = true
				view.Aliases = append(view.Aliases, alias)
			}
		}
	}
	if raw, ok := metadata["identity-property"]; ok {
		name, valid := raw.(string)
		if !valid || !readableScalar(projector.document, properties[name]) {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "identity-property %q is not a readable scalar property", fmt.Sprint(raw)))
		} else {
			view.IdentityProperty = name
		}
	}
	if raw, ok := metadata["columns"]; ok {
		columns, err := projector.parseColumns(raw, properties, source, view.ID)
		if err != nil {
			projector.fatal = append(projector.fatal, err)
		} else {
			view.Columns = columns
		}
	}
	if raw, ok := metadata["default-sort"]; ok {
		name, valid := raw.(string)
		if !valid || !readableScalar(projector.document, properties[name]) {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "default-sort %q is not a readable scalar property", fmt.Sprint(raw)))
		} else if _, columnsExplicit := metadata["columns"]; columnsExplicit && !columnPresent(view.Columns, name) {
			projector.fatal = append(projector.fatal, diagnostic(source, "view "+view.ID, "default-sort %q is absent from explicit columns", name))
		} else {
			view.DefaultSort = name
		}
	} else if len(view.Columns) > 0 {
		view.DefaultSort = view.Columns[0].Property
	}
}

func (projector *projection) parseColumns(raw any, properties map[string]*ir.Property, source ir.SourceLocation, viewID string) ([]tui.Column, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, diagnostic(source, "view "+viewID, "columns must be a non-empty array")
	}
	var failures []error
	var result []tui.Column
	seen := make(map[string]bool)
	for index, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			failures = append(failures, diagnostic(source, "view "+viewID, "columns[%d] must be an object", index))
			continue
		}
		for key := range object {
			if key != "property" && key != "label" && key != "priority" {
				failures = append(failures, diagnostic(source, "view "+viewID, "columns[%d] has unknown field %q", index, key))
			}
		}
		property, propertyOK := object["property"].(string)
		label, labelOK := object["label"].(string)
		priority, priorityOK := integer(object["priority"])
		if !propertyOK || !readableScalar(projector.document, properties[property]) || seen[property] {
			failures = append(failures, diagnostic(source, "view "+viewID, "columns[%d].property %q is missing, unreadable, non-scalar, or duplicated", index, fmt.Sprint(object["property"])))
		}
		if !labelOK || strings.TrimSpace(label) == "" || tui.HasTerminalControl(label) {
			failures = append(failures, diagnostic(source, "view "+viewID, "columns[%d].label must be a non-empty terminal-safe string", index))
		}
		if !priorityOK {
			failures = append(failures, diagnostic(source, "view "+viewID, "columns[%d].priority must be an integer", index))
		}
		if propertyOK && labelOK && priorityOK && readableScalar(projector.document, properties[property]) && !seen[property] && !tui.HasTerminalControl(label) && strings.TrimSpace(label) != "" {
			seen[property] = true
			result = append(result, projector.presentationColumn(property, label, priority, properties[property]))
		}
	}
	return result, errors.Join(failures...)
}

func (projector *projection) projectRelationships() {
	operationViews := make(map[string][]string)
	for _, view := range projector.descriptor.Views {
		for _, operationID := range view.OperationIDs {
			operationViews[operationID] = append(operationViews[operationID], view.ID)
		}
	}
	for _, relationship := range projector.document.Relationships {
		sourceID, targetID := projector.viewIDs[relationship.SourceViewID], projector.viewIDs[relationship.TargetViewID]
		if sourceID == "" && len(operationViews[relationship.SourceOperationID]) == 1 {
			sourceID = operationViews[relationship.SourceOperationID][0]
		}
		if targetID == "" && len(operationViews[relationship.TargetOperationID]) == 1 {
			targetID = operationViews[relationship.TargetOperationID][0]
		}
		if sourceID == "" || targetID == "" || projector.views[sourceID] == nil || projector.views[targetID] == nil {
			projector.warnings = append(projector.warnings, fmt.Sprintf("relationship %s is not navigable because its source or target view is unresolved", relationship.Name))
			continue
		}
		targetOperation := projector.irOperations[relationship.TargetOperationID]
		if targetOperation == nil {
			projector.warnings = append(projector.warnings, fmt.Sprintf("relationship %s is not navigable because target operation %s is unresolved", relationship.Name, relationship.TargetOperationID))
			continue
		}
		edge := tui.Edge{
			ID:   sourceID + "->" + targetID + ":" + relationship.Name,
			Name: relationship.Name, SourceViewID: sourceID, TargetViewID: targetID,
			SourceOperationID: relationship.SourceOperationID, TargetOperationID: relationship.TargetOperationID,
			Provenance: string(relationship.Provenance), Navigable: true,
		}
		mappings := make(map[string]any)
		for _, mapping := range relationship.ParameterMappings {
			mappings[mapping.Target] = mapping.Expression
		}
		projector.bindEdge(&edge, targetOperation, projector.irOperations[relationship.SourceOperationID], mappings, relationship.Provenance == ir.RelationshipExplicit)
		projector.descriptor.Edges = append(projector.descriptor.Edges, edge)
	}
}

func (projector *projection) projectCollectionItemEdges() {
	for _, collection := range projector.descriptor.Views {
		if collection.ListOperationID == "" || collection.IdentityProperty == "" {
			continue
		}
		var candidates []tui.View
		collectionIR := projector.irViews[collection.ID]
		for _, item := range projector.descriptor.Views {
			if item.Kind != "item" || item.GetOperationID == "" || item.SchemaRef != collection.SchemaRef {
				continue
			}
			itemIR := projector.irViews[item.ID]
			if itemIR != nil && collectionIR != nil && strings.HasPrefix(itemIR.Path, strings.TrimSuffix(collectionIR.Path, "/")+"/") && len(itemIR.ScopeParameters) == len(collectionIR.ScopeParameters) {
				candidates = append(candidates, item)
			}
		}
		if len(candidates) != 1 {
			if len(candidates) > 1 {
				projector.warnings = append(projector.warnings, fmt.Sprintf("view %s has multiple collection-to-item candidates and requires an explicit relationship", collection.ID))
			}
			continue
		}
		item := candidates[0]
		edge := tui.Edge{
			ID: collection.ID + "->" + item.ID + ":item", Name: "details",
			SourceViewID: collection.ID, TargetViewID: item.ID,
			SourceOperationID: collection.ListOperationID, TargetOperationID: item.GetOperationID,
			Provenance: "collection-item", Navigable: true,
		}
		projector.bindEdge(&edge, projector.irOperations[item.GetOperationID], projector.irOperations[collection.ListOperationID], nil, false)
		projector.descriptor.Edges = append(projector.descriptor.Edges, edge)
	}
}

func (projector *projection) bindEdge(edge *tui.Edge, target, source *ir.Operation, mappings map[string]any, explicit bool) {
	sourceParameters := make(map[string]bool)
	if source != nil {
		for _, parameter := range source.PathParameters {
			sourceParameters[parameter.Name] = true
		}
	}
	remaining := make([]string, 0)
	for _, parameter := range target.PathParameters {
		if expression, ok := mappings[parameter.Name]; ok {
			binding, err := mappedBinding(parameter.Name, expression)
			if err != nil {
				projector.fatal = append(projector.fatal, diagnostic(target.Source, "relationship "+edge.Name, "%v", err))
				edge.Navigable = false
				edge.Diagnostic = err.Error()
				continue
			}
			edge.Bindings = append(edge.Bindings, binding)
			continue
		}
		if sourceParameters[parameter.Name] {
			edge.Bindings = append(edge.Bindings, tui.Binding{Target: parameter.Name, SourceKind: "frame-path", Source: parameter.Name})
			continue
		}
		remaining = append(remaining, parameter.Name)
	}
	if len(remaining) == 1 {
		sourceView := projector.views[edge.SourceViewID]
		if sourceView != nil && sourceView.IdentityProperty != "" {
			edge.Bindings = append(edge.Bindings, tui.Binding{Target: remaining[0], SourceKind: "row-property", Source: sourceView.IdentityProperty})
			remaining = nil
		}
	}
	if len(remaining) > 0 {
		edge.Navigable = false
		edge.Diagnostic = fmt.Sprintf("target operation %s has unsatisfied path parameters: %s", target.ID, strings.Join(remaining, ", "))
		if explicit {
			projector.fatal = append(projector.fatal, diagnostic(target.Source, "relationship "+edge.Name, "%s", edge.Diagnostic))
		} else {
			projector.warnings = append(projector.warnings, edge.Diagnostic)
		}
	}
	sort.Slice(edge.Bindings, func(i, j int) bool { return edge.Bindings[i].Target < edge.Bindings[j].Target })
}

func (projector *projection) applyExplicitPrecedence() {
	explicit := make(map[string]bool)
	for _, edge := range projector.descriptor.Edges {
		if edge.Provenance == string(ir.RelationshipExplicit) {
			explicit[edge.SourceViewID+"\x00"+edge.TargetViewID] = true
		}
	}
	filtered := projector.descriptor.Edges[:0]
	for _, edge := range projector.descriptor.Edges {
		pair := edge.SourceViewID + "\x00" + edge.TargetViewID
		if edge.Provenance == string(ir.RelationshipInferred) && explicit[pair] {
			continue
		}
		filtered = append(filtered, edge)
	}
	projector.descriptor.Edges = filtered
}

func (projector *projection) projectOperationPresentation() {
	for _, operation := range projector.document.Operations {
		if operation.ID == "" {
			continue
		}
		if projector.isCollectionList(operation.ID) {
			continue
		}
		projected := projector.operations[operation.ID]
		if projected == nil {
			continue
		}
		value, ok := operation.Extensions[tuiExtension]
		var metadata map[string]any
		if ok {
			metadata, ok = value.Value.(map[string]any)
		}
		if value.Value != nil && !ok {
			projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "x-trex-tui must be an object"))
			continue
		}
		for _, name := range sortedMetadataKeys(metadata) {
			if name == "visibility" {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "x-trex-tui field %q is unsupported", name))
			} else if name != "label" && name != "hotkey" && name != "confirmation" {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "unknown x-trex-tui field %q", name))
			}
		}
		if raw, present := metadata["label"]; present {
			label, valid := safeNonemptyString(raw)
			if !valid {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "label must be a non-empty terminal-safe string"))
			} else {
				projected.Presentation.Label = label
			}
		}
		if raw, present := metadata["hotkey"]; present {
			hotkey, valid := raw.(string)
			if !valid || !hotkeyPattern.MatchString(hotkey) {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "hotkey must match [a-z0-9] or ctrl-[a-z]"))
			} else if tui.DefaultKeyRegistry().Reserved(hotkey) {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "hotkey %q conflicts with the shared keybinding registry", hotkey))
			} else {
				projected.Presentation.Hotkey = hotkey
			}
		}
		if raw, present := metadata["confirmation"]; present {
			confirmation, err := parseConfirmation(raw, actionProjectionLabel(*projected), operation.Method == "DELETE")
			if err != nil {
				projector.fatal = append(projector.fatal, diagnostic(value.Source, "operation "+operation.ID, "confirmation: %v", err))
			} else {
				projected.Presentation.Confirmation = confirmation
			}
		}
		if operation.Method == "DELETE" {
			if projected.Presentation.Confirmation == nil {
				projected.Presentation.Confirmation = defaultConfirmation(actionProjectionLabel(*projected), true)
			}
			projected.Presentation.Confirmation.Destructive = true
		}
	}
}

func (projector *projection) validateOperationHotkeyConflicts() {
	for _, view := range projector.descriptor.Views {
		owners := make(map[string]*tui.Operation)
		seenOperations := make(map[string]bool)
		operationIDs := append([]string(nil), view.OperationIDs...)
		if view.Kind == "collection" {
			for _, edge := range projector.descriptor.Outgoing(view.ID) {
				target := projector.views[edge.TargetViewID]
				if target != nil && target.Kind == "item" && target.SchemaRef == view.SchemaRef {
					operationIDs = append(operationIDs, target.OperationIDs...)
				}
			}
		}
		for _, operationID := range operationIDs {
			if seenOperations[operationID] {
				continue
			}
			seenOperations[operationID] = true
			operation := projector.operations[operationID]
			if operation == nil || operation.Presentation.Hotkey == "" || slices.Contains(operation.Capabilities, "list") || slices.Contains(operation.Capabilities, "get") {
				continue
			}
			hotkey := operation.Presentation.Hotkey
			if previous := owners[hotkey]; previous != nil {
				previousLocation := previous.Source.File
				if previous.Source.Pointer != "" {
					previousLocation += "#" + previous.Source.Pointer
				}
				source := projector.irOperations[operation.ID].Source
				projector.fatal = append(projector.fatal, diagnostic(source, "operation "+operation.ID, "hotkey %q conflicts with operation %s at %s on view %s", hotkey, previous.ID, previousLocation, view.ID))
				continue
			}
			owners[hotkey] = operation
		}
	}
}

func safeNonemptyString(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok && strings.TrimSpace(text) != "" && !tui.HasTerminalControl(text)
}

func parseConfirmation(raw any, label string, destructive bool) (*tui.Confirmation, error) {
	metadata, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be an object")
	}
	confirmation := defaultConfirmation(label, destructive)
	for _, name := range sortedMetadataKeys(metadata) {
		if name != "title" && name != "message" && name != "destructive" {
			return nil, fmt.Errorf("unknown field %q", name)
		}
	}
	if rawTitle, present := metadata["title"]; present {
		title, valid := safeNonemptyString(rawTitle)
		if !valid {
			return nil, fmt.Errorf("title must be a non-empty terminal-safe string")
		}
		confirmation.Title = title
	}
	if rawMessage, present := metadata["message"]; present {
		message, valid := safeNonemptyString(rawMessage)
		if !valid {
			return nil, fmt.Errorf("message must be a non-empty terminal-safe string")
		}
		confirmation.Message = message
	}
	if rawDestructive, present := metadata["destructive"]; present {
		value, valid := rawDestructive.(bool)
		if !valid {
			return nil, fmt.Errorf("destructive must be a boolean")
		}
		confirmation.Destructive = value || destructive
	}
	return confirmation, nil
}

func defaultConfirmation(label string, destructive bool) *tui.Confirmation {
	title := "Confirm"
	if destructive {
		title = "Delete"
	}
	message := strings.TrimSpace(label)
	return &tui.Confirmation{Title: title, Message: strings.TrimSuffix(message, "?") + "?", Destructive: destructive}
}

func actionProjectionLabel(operation tui.Operation) string {
	if operation.Presentation.Label != "" {
		return operation.Presentation.Label
	}
	if operation.Summary != "" {
		return operation.Summary
	}
	return operation.ID
}

func (projector *projection) isCollectionList(operationID string) bool {
	for _, view := range projector.descriptor.Views {
		if view.ListOperationID == operationID {
			return true
		}
	}
	return false
}

func (projector *projection) validateAliases() {
	groups := [][]string{{}}
	for _, view := range projector.descriptor.Views {
		if view.ListOperationID != "" && len(view.ScopeParameters) == 0 {
			groups[0] = append(groups[0], view.ID)
		}
	}
	bySource := make(map[string][]string)
	for _, edge := range projector.descriptor.Edges {
		if edge.Navigable {
			bySource[edge.SourceViewID] = append(bySource[edge.SourceViewID], edge.TargetViewID)
		}
	}
	for _, group := range bySource {
		groups = append(groups, group)
	}
	for _, group := range groups {
		seen := make(map[string]string)
		for _, viewID := range group {
			view := projector.views[viewID]
			if view == nil {
				continue
			}
			for _, alias := range view.Aliases {
				if prior := seen[alias]; prior != "" && prior != view.ID {
					projector.fatal = append(projector.fatal, fmt.Errorf("views %s and %s have conflicting simultaneously addressable alias %q", prior, view.ID, alias))
				}
				seen[alias] = view.ID
			}
		}
	}
}

func (projector *projection) recordUnaddressableScopes() {
	incoming := make(map[string]bool)
	for _, edge := range projector.descriptor.Edges {
		if edge.Navigable {
			incoming[edge.TargetViewID] = true
		}
	}
	for _, view := range projector.descriptor.Views {
		if view.ListOperationID != "" && len(view.ScopeParameters) > 0 && !incoming[view.ID] {
			projector.warnings = append(projector.warnings, fmt.Sprintf("scoped view %s is not navigable; an explicit relationship may be required", view.ID))
		}
	}
}

func (projector *projection) finish() {
	sort.Slice(projector.descriptor.Views, func(i, j int) bool { return projector.descriptor.Views[i].ID < projector.descriptor.Views[j].ID })
	sort.Slice(projector.descriptor.Operations, func(i, j int) bool {
		return projector.descriptor.Operations[i].ID < projector.descriptor.Operations[j].ID
	})
	sort.Slice(projector.descriptor.Edges, func(i, j int) bool { return projector.descriptor.Edges[i].ID < projector.descriptor.Edges[j].ID })
	sort.Slice(projector.descriptor.SecuritySchemes, func(i, j int) bool {
		return projector.descriptor.SecuritySchemes[i].Name < projector.descriptor.SecuritySchemes[j].Name
	})
	sort.Strings(projector.warnings)
}

func (projector *projection) readableScalarProperties(schemaRef string) map[string]*ir.Property {
	result := make(map[string]*ir.Property)
	for name, property := range projector.document.EffectiveProperties(schemaRef) {
		if readableScalar(projector.document, property) {
			result[name] = property
		}
	}
	return result
}

func (projector *projection) presentationColumn(property, label string, priority int, source *ir.Property) tui.Column {
	column := tui.Column{Property: property, Label: label, Priority: priority}
	if source == nil || source.Schema == nil {
		return column
	}
	schema := projector.document.Schema(source.Schema.Ref)
	if schema == nil {
		return column
	}
	if len(schema.Types) > 0 {
		column.Type = schema.Types[0]
	}
	column.Format = schema.Format
	return column
}

func readableScalar(document *ir.Document, property *ir.Property) bool {
	if property == nil || property.WriteOnly || property.Schema == nil {
		return false
	}
	schema := document.Schema(property.Schema.Ref)
	if schema == nil || len(schema.Types) == 0 {
		return false
	}
	switch schema.Types[0] {
	case "string", "integer", "number", "boolean":
		return true
	default:
		return false
	}
}

func (projector *projection) parameter(parameter *ir.Parameter) tui.Parameter {
	result := tui.Parameter{Name: parameter.Name, In: parameter.In, Required: parameter.Required, Style: parameter.Style, Explode: parameter.Explode, AllowReserved: parameter.AllowReserved, Description: parameter.Description}
	if parameter.Schema != nil {
		if schema := projector.document.Schema(parameter.Schema.Ref); schema != nil {
			if len(schema.Types) > 0 {
				result.Type = schema.Types[0]
			}
			result.Format, result.Pattern = schema.Format, schema.Pattern
			result.Enum = append([]any(nil), schema.Enum...)
			result.Default = schema.Default
		}
	}
	return result
}

func (projector *projection) requestBody(operation *ir.Operation) *tui.RequestBody {
	if operation.RequestBody == nil {
		return nil
	}
	var media *ir.MediaType
	for _, candidate := range operation.RequestBody.Content {
		if candidate.ContentType == "application/json" || candidate.ContentType == "application/merge-patch+json" {
			media = candidate
			break
		}
	}
	if media == nil {
		if len(operation.RequestBody.Content) > 0 {
			projector.fatal = append(projector.fatal, diagnostic(operation.RequestBody.Source, "operation "+operation.ID, "request body has no supported JSON content type"))
		}
		return nil
	}
	result := &tui.RequestBody{Required: operation.RequestBody.Required, ContentType: media.ContentType}
	if media.Schema == nil {
		return result
	}
	properties := projector.document.EffectiveProperties(media.Schema.Ref)
	for _, name := range sortedPropertyNames(properties) {
		property := properties[name]
		if property.ReadOnly || property.Schema == nil {
			continue
		}
		field := tui.InputField{Name: name, Required: property.Required, ReadOnly: property.ReadOnly, WriteOnly: property.WriteOnly, Description: property.Description}
		if schema := projector.document.Schema(property.Schema.Ref); schema != nil {
			if len(schema.Types) > 0 {
				field.Type = schema.Types[0]
			}
			field.Format = schema.Format
			field.Enum = append([]any(nil), schema.Enum...)
			field.Default = schema.Default
		}
		result.Fields = append(result.Fields, field)
	}
	return result
}

func (projector *projection) responseShape(operation *ir.Operation) tui.ResponseShape {
	shape := tui.ResponseShape{Stream: operation.Capabilities.Has(ir.CapabilityStream)}
	for _, response := range operation.Responses {
		if !isSuccess(response.Status) {
			continue
		}
		for _, content := range response.Content {
			if shape.ContentType == "" || strings.Contains(content.ContentType, "json") || content.ContentType == "text/event-stream" {
				shape.ContentType = content.ContentType
				if content.Schema != nil {
					shape.ItemsPointer = projector.itemsPointer(content.Schema.Ref)
				}
			}
		}
	}
	return shape
}

func (projector *projection) itemsPointer(schemaRef string) string {
	schema := projector.document.Schema(schemaRef)
	if schema == nil {
		return ""
	}
	if schema.Items != nil {
		return ""
	}
	properties := projector.document.EffectiveProperties(schemaRef)
	for _, name := range []string{"items", "data", "results"} {
		property := properties[name]
		if property == nil || property.Schema == nil {
			continue
		}
		propertySchema := projector.document.Schema(property.Schema.Ref)
		if propertySchema != nil && propertySchema.Items != nil {
			return "/" + escapeJSONPointer(name)
		}
	}
	return ""
}

func (projector *projection) security(operation *ir.Operation) tui.EffectiveSecurity {
	if operation.Security.State == ir.SecurityNone {
		return tui.EffectiveSecurity{None: true}
	}
	requirements := operation.Security.Requirements
	if operation.Security.State == ir.SecurityInherited {
		requirements = projector.document.Security
	}
	if len(requirements) == 0 {
		return tui.EffectiveSecurity{None: true}
	}
	var supported []tui.SecurityAlternative
	var declared []string
	for _, alternative := range requirements {
		projected := tui.SecurityAlternative{}
		valid := true
		if len(alternative.Schemes) == 0 {
			supported = append(supported, projected)
			continue
		}
		for _, use := range alternative.Schemes {
			declared = append(declared, use.Name)
			scheme := securityScheme(projector.document, use.Name)
			if scheme == nil || scheme.Type != "http" || !strings.EqualFold(scheme.Scheme, "bearer") {
				valid = false
				continue
			}
			projected.Schemes = append(projected.Schemes, use.Name)
		}
		if valid {
			sort.Strings(projected.Schemes)
			supported = append(supported, projected)
		}
	}
	if len(supported) == 0 {
		sort.Strings(declared)
		projector.fatal = append(projector.fatal, diagnostic(operation.Source, "operation "+operation.ID, "required security has no supported HTTP bearer alternative (declared: %s)", strings.Join(declared, ", ")))
	}
	return tui.EffectiveSecurity{Requirements: supported}
}

func compilePathParts(operation *ir.Operation) ([]tui.PathPart, error) {
	parameters := make(map[string]bool)
	for _, parameter := range operation.PathParameters {
		parameters[parameter.Name] = true
	}
	path := operation.Path
	var result []tui.PathPart
	for len(path) > 0 {
		start := strings.IndexByte(path, '{')
		if start < 0 {
			result = append(result, tui.PathPart{Literal: path})
			break
		}
		if start > 0 {
			result = append(result, tui.PathPart{Literal: path[:start]})
		}
		end := strings.IndexByte(path[start:], '}')
		if end < 0 {
			return nil, fmt.Errorf("path %q has an unterminated parameter", operation.Path)
		}
		end += start
		name := path[start+1 : end]
		if !parameters[name] {
			return nil, fmt.Errorf("path parameter %q is missing", name)
		}
		result = append(result, tui.PathPart{Parameter: name})
		path = path[end+1:]
	}
	return result, nil
}

func extensionMap(operation *ir.Operation) (map[string]any, ir.SourceLocation, bool) {
	if operation == nil {
		return nil, ir.SourceLocation{}, false
	}
	value, ok := operation.Extensions[tuiExtension]
	if !ok {
		return nil, ir.SourceLocation{}, false
	}
	object, objectOK := value.Value.(map[string]any)
	if !objectOK {
		return nil, value.Source, true
	}
	return object, value.Source, true
}

func (projector *projection) reindex() {
	projector.views = make(map[string]*tui.View, len(projector.descriptor.Views))
	for index := range projector.descriptor.Views {
		view := &projector.descriptor.Views[index]
		projector.views[view.ID] = view
	}
	projector.operations = make(map[string]*tui.Operation, len(projector.descriptor.Operations))
	for index := range projector.descriptor.Operations {
		operation := &projector.descriptor.Operations[index]
		projector.operations[operation.ID] = operation
	}
}

func mappedBinding(target string, expression any) (tui.Binding, error) {
	if text, ok := expression.(string); ok {
		if supportedRuntimeExpression(text) {
			return tui.Binding{Target: target, SourceKind: "runtime-expression", Source: text}, nil
		}
		if strings.HasPrefix(text, "$") {
			return tui.Binding{}, fmt.Errorf("target %s uses unsupported runtime expression %q", target, text)
		}
		return tui.Binding{Target: target, SourceKind: "literal", Source: text}, nil
	}
	data, err := json.Marshal(expression)
	if err != nil {
		return tui.Binding{}, fmt.Errorf("target %s has an invalid literal", target)
	}
	if expression == nil {
		return tui.Binding{}, fmt.Errorf("target %s has a null literal", target)
	}
	return tui.Binding{Target: target, SourceKind: "literal", Source: strings.Trim(string(data), `"`)}, nil
}

func supportedRuntimeExpression(expression string) bool {
	switch expression {
	case "$url", "$method", "$statusCode":
		return true
	}
	for _, prefix := range []string{
		"$request.path.",
		"$request.query.",
		"$request.header.",
		"$response.header.",
	} {
		if strings.HasPrefix(expression, prefix) && len(expression) > len(prefix) {
			return true
		}
	}
	for _, prefix := range []string{"$request.body", "$response.body"} {
		if expression == prefix || expression == prefix+"#" || strings.HasPrefix(expression, prefix+"#/") {
			return true
		}
	}
	return false
}

func successStatuses(operation *ir.Operation) []string {
	var result []string
	for _, response := range operation.Responses {
		if isSuccess(response.Status) {
			result = append(result, response.Status)
		}
	}
	sort.Strings(result)
	return result
}

func isSuccess(status string) bool {
	if strings.HasPrefix(status, "2") {
		return true
	}
	value, err := strconv.Atoi(status)
	return err == nil && value >= 200 && value < 300
}

func expandServer(server *ir.Server) string {
	if server == nil {
		return ""
	}
	result := server.URL
	keys := make([]string, 0, len(server.Variables))
	for key := range server.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = strings.ReplaceAll(result, "{"+key+"}", server.Variables[key].Default)
	}
	return result
}

func securityScheme(document *ir.Document, name string) *ir.SecurityScheme {
	for _, scheme := range document.SecuritySchemes {
		if scheme.Name == name {
			return scheme
		}
	}
	return nil
}

func diagnostic(source ir.SourceLocation, context, format string, arguments ...any) error {
	location := source.File
	if source.Pointer != "" {
		location += "#" + source.Pointer
	}
	if location == "" {
		location = "OpenAPI"
	}
	return fmt.Errorf("%s: %s: %s", location, context, fmt.Sprintf(format, arguments...))
}

func stringSlice(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array of strings")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("must contain only strings")
		}
		result = append(result, text)
	}
	return result, nil
}

func integer(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), typed == float64(int(typed))
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	default:
		return 0, false
	}
}

func sortedPropertyNames(values map[string]*ir.Property) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMetadataKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func columnPresent(columns []tui.Column, name string) bool {
	for _, column := range columns {
		if column.Property == name {
			return true
		}
	}
	return false
}

func escapeJSONPointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func stableViewID(view *ir.ResourceView, schema *ir.Schema) string {
	return string(view.Kind) + ":" + view.Path + ":" + stableSchemaRef(schema)
}

func stableSchemaRef(schema *ir.Schema) string {
	if schema.Name != "" {
		return "#/schemas/" + escapeJSONPointer(schema.Name)
	}
	if index := strings.IndexByte(schema.Ref, '#'); index >= 0 {
		return schema.Ref[index:]
	}
	return schema.Ref
}
