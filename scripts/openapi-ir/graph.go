package ir

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func (normalizer *normalizer) buildSchemaUsesAndGraph() error {
	views := make(map[string]*ResourceView)
	operationViews := make(map[string][]*ResourceView)
	for _, operation := range normalizer.document.Operations {
		stream := operationIsStream(operation)
		for _, parameter := range operation.Parameters {
			if parameter.Schema != nil {
				normalizer.addUse(operation.ID, parameter.Schema.Ref, SchemaRoleParameter, parameter.In+":"+parameter.Name)
			}
		}
		if operation.RequestBody != nil {
			for _, mediaType := range operation.RequestBody.Content {
				if mediaType.Schema != nil {
					normalizer.addUse(operation.ID, mediaType.Schema.Ref, SchemaRoleRequest, mediaType.ContentType)
				}
			}
		}
		for _, response := range operation.Responses {
			for _, mediaType := range response.Content {
				if mediaType.Schema == nil {
					continue
				}
				role := SchemaRoleResponse
				if responseIsError(response.Status) {
					role = SchemaRoleError
				} else if stream {
					role = SchemaRoleEvent
				}
				normalizer.addUse(operation.ID, mediaType.Schema.Ref, role, response.Status+":"+mediaType.ContentType)
				if itemRef := normalizer.listItemReference(mediaType.Schema.Ref, make(map[string]bool)); itemRef != "" {
					normalizer.addUse(operation.ID, itemRef, SchemaRoleListItem, response.Status+":"+mediaType.ContentType)
				}
			}
		}

		schemaRef, isList := normalizer.representedSchema(operation, stream)
		kind := resourceKind(operation.Path, isList)
		operation.Capabilities = operationCapabilities(operation, kind, stream)
		if schemaRef == "" {
			continue
		}
		viewPath := operation.Path
		key := resourceViewID(kind, viewPath)
		view := views[key]
		if view == nil {
			view = &ResourceView{
				ID:   key,
				Kind: kind, Path: viewPath, SchemaRef: schemaRef,
				ScopeParameters: scopeParameterNames(operation.PathParameters, kind),
			}
			views[key] = view
			normalizer.document.ResourceViews = append(normalizer.document.ResourceViews, view)
		} else if view.SchemaRef != schemaRef {
			return newDiagnostic(operation.Source, "operation "+operation.ID, "resource view %q has conflicting represented schemas %q and %q", key, view.SchemaRef, schemaRef)
		}
		attachOperationToView(operation, view, operationViews)
		if len(view.Extensions) == 0 && len(operation.Extensions) > 0 {
			view.Extensions = operation.Extensions
		}
	}

	for _, operation := range normalizer.document.Operations {
		if len(operationViews[operation.ID]) > 0 {
			continue
		}
		if view := normalizer.operationOwnerView(operation); view != nil {
			attachOperationToView(operation, view, operationViews)
		}
	}
	for _, view := range normalizer.document.ResourceViews {
		sort.Strings(view.OperationIDs)
		view.Capabilities = sortCapabilities(view.Capabilities)
	}
	if err := normalizer.buildExplicitRelationships(operationViews); err != nil {
		return err
	}
	normalizer.inferRelationships()
	return nil
}

func (normalizer *normalizer) addUse(operationID, schemaRef string, role SchemaRole, context string) {
	if schemaRef == "" {
		return
	}
	normalizer.document.SchemaUses = append(normalizer.document.SchemaUses, &SchemaUse{
		OperationID: operationID, SchemaRef: schemaRef, Role: role, Context: context,
	})
}

func (normalizer *normalizer) representedSchema(operation *Operation, stream bool) (string, bool) {
	if stream {
		return "", false
	}
	for _, response := range operation.Responses {
		if !responseIsSuccess(response.Status) {
			continue
		}
		for _, content := range response.Content {
			if content.Schema == nil || !isJSONContentType(content.ContentType) {
				continue
			}
			if item := normalizer.listItemReference(content.Schema.Ref, make(map[string]bool)); item != "" {
				return item, true
			}
			return content.Schema.Ref, false
		}
	}
	return "", false
}

func isJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func (normalizer *normalizer) listItemReference(ref string, visiting map[string]bool) string {
	if visiting[ref] {
		return ""
	}
	visiting[ref] = true
	defer delete(visiting, ref)
	schema := normalizer.schemas[ref]
	if schema == nil {
		return ""
	}
	if schema.Items != nil {
		return schema.Items.Ref
	}
	properties := normalizer.EffectiveProperties(ref)
	for _, candidate := range []string{"items", "data", "results"} {
		property := properties[candidate]
		if property == nil || property.Schema == nil {
			continue
		}
		propertySchema := normalizer.schemas[property.Schema.Ref]
		if propertySchema != nil && propertySchema.Items != nil {
			return propertySchema.Items.Ref
		}
	}
	for _, composed := range schema.AllOf {
		if item := normalizer.listItemReference(composed.Ref, visiting); item != "" {
			return item
		}
	}
	return ""
}

func (normalizer *normalizer) EffectiveProperties(ref string) map[string]*Property {
	result := make(map[string]*Property)
	normalizer.collectProperties(ref, result, make(map[string]bool))
	return result
}

func (normalizer *normalizer) collectProperties(ref string, result map[string]*Property, visiting map[string]bool) {
	if visiting[ref] {
		return
	}
	visiting[ref] = true
	defer delete(visiting, ref)
	schema := normalizer.schemas[ref]
	if schema == nil {
		return
	}
	for _, composed := range schema.AllOf {
		normalizer.collectProperties(composed.Ref, result, visiting)
	}
	for name, property := range schema.Properties {
		result[name] = property
	}
}

func (document *Document) EffectiveProperties(ref string) map[string]*Property {
	byRef := make(map[string]*Schema, len(document.Schemas))
	for _, schema := range document.Schemas {
		byRef[schema.Ref] = schema
	}
	result := make(map[string]*Property)
	var collect func(string, map[string]bool)
	collect = func(current string, visiting map[string]bool) {
		if visiting[current] {
			return
		}
		visiting[current] = true
		defer delete(visiting, current)
		schema := byRef[current]
		if schema == nil {
			return
		}
		for _, composed := range schema.AllOf {
			collect(composed.Ref, visiting)
		}
		for name, property := range schema.Properties {
			result[name] = property
		}
	}
	collect(ref, make(map[string]bool))
	return result
}

func resourceViewID(kind ResourceViewKind, path string) string {
	return string(kind) + ":" + path
}

func attachOperationToView(operation *Operation, view *ResourceView, operationViews map[string][]*ResourceView) {
	for _, operationID := range view.OperationIDs {
		if operationID == operation.ID {
			return
		}
	}
	view.OperationIDs = append(view.OperationIDs, operation.ID)
	view.Capabilities = append(view.Capabilities, operation.Capabilities...)
	operationViews[operation.ID] = append(operationViews[operation.ID], view)
}

func (normalizer *normalizer) operationOwnerView(operation *Operation) *ResourceView {
	if exact := uniqueViewAtPath(normalizer.document.ResourceViews, operation.Path); exact != nil {
		return exact
	}
	if strings.Contains(operation.Path, ":") {
		base := operation.Path[:strings.LastIndex(operation.Path, ":")]
		if actionOwner := uniqueViewAtPath(normalizer.document.ResourceViews, base); actionOwner != nil {
			return actionOwner
		}
	}
	if operation.Capabilities.Has(CapabilityStream) {
		var candidates []*ResourceView
		longest := -1
		for _, view := range normalizer.document.ResourceViews {
			if view.Kind != ResourceItem || !pathContainsPrefix(operation.Path, view.Path) {
				continue
			}
			if len(view.Path) > longest {
				candidates = []*ResourceView{view}
				longest = len(view.Path)
			} else if len(view.Path) == longest {
				candidates = append(candidates, view)
			}
		}
		if len(candidates) == 1 {
			return candidates[0]
		}
	}
	return nil
}

func uniqueViewAtPath(views []*ResourceView, path string) *ResourceView {
	var result *ResourceView
	for _, view := range views {
		if view.Path != path {
			continue
		}
		if result != nil {
			return nil
		}
		result = view
	}
	return result
}

func (normalizer *normalizer) buildExplicitRelationships(operationViews map[string][]*ResourceView) error {
	for _, operation := range normalizer.document.Operations {
		for _, response := range operation.Responses {
			for _, link := range response.Links {
				targetOperationID, err := normalizer.resolveLinkTarget(link)
				if err != nil {
					return err
				}
				sourceViewID, err := uniqueOperationViewID(operationViews[operation.ID], link.Source, "source", operation.ID)
				if err != nil {
					return err
				}
				targetViewID, err := uniqueOperationViewID(operationViews[targetOperationID], link.Source, "target", targetOperationID)
				if err != nil {
					return err
				}
				normalizer.document.Relationships = append(normalizer.document.Relationships, &Relationship{
					Name: link.Name, SourceOperationID: operation.ID, SourceResponseStatus: response.Status,
					TargetOperationID:  targetOperationID,
					TargetOperationRef: link.TargetOperationRef, SourceViewID: sourceViewID, TargetViewID: targetViewID,
					ParameterMappings: append([]ParameterMapping(nil), link.Parameters...),
					Provenance:        RelationshipExplicit, Source: link.Source,
				})
			}
		}
	}
	return nil
}

func (normalizer *normalizer) resolveLinkTarget(link *OperationLink) (string, error) {
	hasID := strings.TrimSpace(link.TargetOperationID) != ""
	hasRef := strings.TrimSpace(link.TargetOperationRef) != ""
	if hasID == hasRef {
		return "", newDiagnostic(link.Source, "link "+link.Name, "exactly one of operationId or operationRef is required")
	}
	if hasID {
		if normalizer.document.Operation(link.TargetOperationID) == nil {
			return "", newDiagnostic(link.Source, "link "+link.Name, "target operationId %q was not found", link.TargetOperationID)
		}
		return link.TargetOperationID, nil
	}
	targetOperationID, err := normalizer.scan.resolveOperationReference(link.TargetOperationRef, link.Source)
	if err != nil {
		return "", newDiagnostic(link.Source, "link "+link.Name, "resolve operationRef %q: %v", link.TargetOperationRef, err)
	}
	return targetOperationID, nil
}

func uniqueOperationViewID(views []*ResourceView, source SourceLocation, endpoint, operationID string) (string, error) {
	if len(views) == 0 {
		return "", nil
	}
	if len(views) > 1 {
		ids := make([]string, 0, len(views))
		for _, view := range views {
			ids = append(ids, view.ID)
		}
		sort.Strings(ids)
		return "", newDiagnostic(source, "operation "+operationID, "%s relationship endpoint belongs to multiple resource views: %s", endpoint, strings.Join(ids, ", "))
	}
	return views[0].ID, nil
}

func (normalizer *normalizer) inferRelationships() {
	explicitPairs := make(map[string]bool)
	for _, relationship := range normalizer.document.Relationships {
		if relationship.SourceViewID != "" && relationship.TargetViewID != "" {
			explicitPairs[relationship.SourceViewID+"\x00"+relationship.TargetViewID] = true
		}
	}
	for _, child := range normalizer.document.ResourceViews {
		if child.Kind != ResourceCollection || len(child.ScopeParameters) == 0 {
			continue
		}
		var candidates []*ResourceView
		for _, parent := range normalizer.document.ResourceViews {
			if parent.Kind != ResourceItem || parent.ID == child.ID {
				continue
			}
			if pathContainsPrefix(child.Path, parent.Path) {
				candidates = append(candidates, parent)
			}
		}
		if len(candidates) != 1 {
			continue
		}
		parent := candidates[0]
		source := normalizer.uniqueCapabilityOperation(parent, CapabilityGet)
		target := normalizer.uniqueCapabilityOperation(child, CapabilityList)
		if source == nil || target == nil {
			continue
		}
		if explicitPairs[parent.ID+"\x00"+child.ID] {
			continue
		}
		mappings, complete := structuralPathMappings(source, target)
		if !complete {
			continue
		}
		normalizer.document.Relationships = append(normalizer.document.Relationships, &Relationship{
			Name: "contains", SourceOperationID: source.ID, TargetOperationID: target.ID,
			SourceViewID: parent.ID, TargetViewID: child.ID, ParameterMappings: mappings,
			Provenance: RelationshipInferred, Source: target.Source,
		})
	}
}

func (normalizer *normalizer) uniqueCapabilityOperation(view *ResourceView, capability Capability) *Operation {
	var result *Operation
	for _, operationID := range view.OperationIDs {
		operation := normalizer.document.Operation(operationID)
		if operation == nil || operation.Method != "GET" || !operation.Capabilities.Has(capability) {
			continue
		}
		if result != nil {
			return nil
		}
		result = operation
	}
	return result
}

func structuralPathMappings(source, target *Operation) ([]ParameterMapping, bool) {
	sourcePathParameters := make(map[string]*Parameter, len(source.PathParameters))
	for _, parameter := range source.PathParameters {
		sourcePathParameters[parameter.Name] = parameter
	}
	mappings := make([]ParameterMapping, 0, len(target.PathParameters))
	for _, parameter := range target.PathParameters {
		sourceParameter := sourcePathParameters[parameter.Name]
		if sourceParameter == nil || sourceParameter.In != parameter.In {
			return nil, false
		}
		mappings = append(mappings, ParameterMapping{Target: parameter.Name, Expression: "$request.path." + sourceParameter.Name})
	}
	return mappings, true
}

func operationCapabilities(operation *Operation, kind ResourceViewKind, stream bool) Capabilities {
	if stream {
		return Capabilities{CapabilityStream}
	}
	var result Capabilities
	if strings.Contains(operation.Path, ":") {
		result = append(result, CapabilityAction)
		return sortCapabilities(result)
	}
	switch operation.Method {
	case "GET":
		if kind == ResourceCollection {
			result = append(result, CapabilityList)
		} else {
			result = append(result, CapabilityGet)
		}
	case "POST":
		if kind == ResourceCollection {
			result = append(result, CapabilityCreate)
		} else {
			result = append(result, CapabilityAction)
		}
	case "PATCH", "PUT":
		result = append(result, CapabilityUpdate)
	case "DELETE":
		result = append(result, CapabilityDelete)
	default:
		result = append(result, CapabilityAction)
	}
	return sortCapabilities(result)
}

func resourceKind(path string, responseIsList bool) ResourceViewKind {
	if responseIsList {
		return ResourceCollection
	}
	last := path
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		last = path[index+1:]
	}
	if strings.Contains(last, "{") {
		return ResourceItem
	}
	return ResourceCollection
}

func scopeParameterNames(parameters []*Parameter, kind ResourceViewKind) []string {
	length := len(parameters)
	if kind == ResourceItem && length > 0 {
		length--
	}
	result := make([]string, 0, length)
	for _, parameter := range parameters[:length] {
		result = append(result, parameter.Name)
	}
	return result
}

func operationIsStream(operation *Operation) bool {
	for _, response := range operation.Responses {
		for _, content := range response.Content {
			if content.ContentType == "text/event-stream" || content.ContentType == "application/x-ndjson" || content.ContentType == "application/octet-stream" {
				return true
			}
		}
	}
	return false
}

func responseIsSuccess(status string) bool {
	if strings.HasPrefix(status, "2") {
		return true
	}
	value, err := strconv.Atoi(status)
	return err == nil && value >= 200 && value < 300
}

func responseIsError(status string) bool {
	if strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5") || status == "default" {
		return true
	}
	value, err := strconv.Atoi(status)
	return err == nil && value >= 400
}

func pathContainsPrefix(child, parent string) bool {
	if child == parent {
		return false
	}
	parent = strings.TrimSuffix(parent, "/")
	return strings.HasPrefix(child, parent+"/")
}

func (document *Document) ValidateProjectionNames() error {
	for _, schema := range document.Schemas {
		if schema.Name != "" {
			if err := ValidateComponentName(schema.Name); err != nil {
				return fmt.Errorf("schema %s: %w", schema.Ref, err)
			}
		}
		for name := range schema.Properties {
			if err := ValidateIdentifier(name); err != nil {
				return fmt.Errorf("schema %s property: %w", schema.Ref, err)
			}
		}
	}
	for _, operation := range document.Operations {
		if err := ValidateIdentifier(operation.ID); err != nil {
			return fmt.Errorf("operation %s: %w", operation.ID, err)
		}
		for _, segment := range operation.Segments {
			if segment.Literal != "" {
				if err := ValidateRouteLiteral(segment.Literal); err != nil {
					return fmt.Errorf("operation %s: %w", operation.ID, err)
				}
			}
		}
	}
	return nil
}
