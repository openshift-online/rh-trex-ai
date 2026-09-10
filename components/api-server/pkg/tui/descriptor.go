// Code generated runtime support for TRex TUI descriptors.
package tui

import (
	"encoding/json"
	"fmt"
)

type Descriptor struct {
	Title           string           `json:"title"`
	Servers         []Server         `json:"servers,omitempty"`
	SecuritySchemes []SecurityScheme `json:"securitySchemes,omitempty"`
	Views           []View           `json:"views"`
	Operations      []Operation      `json:"operations"`
	Edges           []Edge           `json:"edges,omitempty"`
	Diagnostics     []string         `json:"diagnostics,omitempty"`
}

type Server struct {
	URL string `json:"url"`
}

type SecurityScheme struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Scheme string `json:"scheme,omitempty"`
}

type View struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	SchemaRef          string   `json:"schemaRef"`
	Label              string   `json:"label"`
	Aliases            []string `json:"aliases,omitempty"`
	IdentityProperty   string   `json:"identityProperty,omitempty"`
	DefaultSort        string   `json:"defaultSort,omitempty"`
	Columns            []Column `json:"columns,omitempty"`
	ScopeParameters    []string `json:"scopeParameters,omitempty"`
	OperationIDs       []string `json:"operationIds"`
	Capabilities       []string `json:"capabilities,omitempty"`
	ListOperationID    string   `json:"listOperationId,omitempty"`
	GetOperationID     string   `json:"getOperationId,omitempty"`
	StreamOperationIDs []string `json:"streamOperationIds,omitempty"`
	Source             Source   `json:"source"`
	FillWidth          bool     `json:"-"`
}

type Column struct {
	Property string `json:"property"`
	Label    string `json:"label"`
	Priority int    `json:"priority"`
	Type     string `json:"type,omitempty"`
	Format   string `json:"format,omitempty"`
}

type Source struct {
	File    string `json:"file,omitempty"`
	Pointer string `json:"pointer,omitempty"`
}

type Operation struct {
	ID              string             `json:"id"`
	Method          string             `json:"method"`
	PathParts       []PathPart         `json:"pathParts"`
	Parameters      []Parameter        `json:"parameters,omitempty"`
	RequestBody     *RequestBody       `json:"requestBody,omitempty"`
	Response        ResponseShape      `json:"response"`
	SuccessStatuses []string           `json:"successStatuses"`
	Servers         []Server           `json:"servers,omitempty"`
	Security        EffectiveSecurity  `json:"security"`
	Capabilities    []string           `json:"capabilities,omitempty"`
	Summary         string             `json:"summary,omitempty"`
	Presentation    ActionPresentation `json:"presentation,omitempty"`
	Source          Source             `json:"-"`
}

type ActionPresentation struct {
	Label        string        `json:"label,omitempty"`
	Hotkey       string        `json:"hotkey,omitempty"`
	Confirmation *Confirmation `json:"confirmation,omitempty"`
}

type Confirmation struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	Destructive bool   `json:"destructive"`
}

type PathPart struct {
	Literal   string `json:"literal,omitempty"`
	Parameter string `json:"parameter,omitempty"`
}

type Parameter struct {
	Name          string `json:"name"`
	In            string `json:"in"`
	Required      bool   `json:"required"`
	Style         string `json:"style"`
	Explode       bool   `json:"explode"`
	AllowReserved bool   `json:"allowReserved,omitempty"`
	Type          string `json:"type,omitempty"`
	Format        string `json:"format,omitempty"`
	Pattern       string `json:"pattern,omitempty"`
	Description   string `json:"description,omitempty"`
	Enum          []any  `json:"enum,omitempty"`
	Default       any    `json:"default,omitempty"`
}

type RequestBody struct {
	Required    bool         `json:"required"`
	ContentType string       `json:"contentType"`
	Fields      []InputField `json:"fields,omitempty"`
}

type InputField struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Format      string `json:"format,omitempty"`
	Required    bool   `json:"required"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
	WriteOnly   bool   `json:"writeOnly,omitempty"`
	Description string `json:"description,omitempty"`
	Enum        []any  `json:"enum,omitempty"`
	Default     any    `json:"default,omitempty"`
}

type ResponseShape struct {
	ContentType  string `json:"contentType,omitempty"`
	ItemsPointer string `json:"itemsPointer,omitempty"`
	Stream       bool   `json:"stream,omitempty"`
}

type EffectiveSecurity struct {
	None         bool                  `json:"none"`
	Requirements []SecurityAlternative `json:"requirements,omitempty"`
}

type SecurityAlternative struct {
	Schemes []string `json:"schemes"`
}

type Edge struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	SourceViewID      string    `json:"sourceViewId"`
	TargetViewID      string    `json:"targetViewId"`
	SourceOperationID string    `json:"sourceOperationId,omitempty"`
	TargetOperationID string    `json:"targetOperationId"`
	Provenance        string    `json:"provenance"`
	Bindings          []Binding `json:"bindings,omitempty"`
	Navigable         bool      `json:"navigable"`
	Diagnostic        string    `json:"diagnostic,omitempty"`
}

type Binding struct {
	Target     string `json:"target"`
	SourceKind string `json:"sourceKind"`
	Source     string `json:"source"`
}

func ParseDescriptor(data []byte) (Descriptor, error) {
	var descriptor Descriptor
	if err := json.Unmarshal(data, &descriptor); err != nil {
		return Descriptor{}, fmt.Errorf("decode generated TUI descriptor: %w", err)
	}
	return descriptor, nil
}

func (descriptor Descriptor) View(id string) *View {
	for index := range descriptor.Views {
		if descriptor.Views[index].ID == id {
			return &descriptor.Views[index]
		}
	}
	return nil
}

func (descriptor Descriptor) Operation(id string) *Operation {
	for index := range descriptor.Operations {
		if descriptor.Operations[index].ID == id {
			return &descriptor.Operations[index]
		}
	}
	return nil
}

func (descriptor Descriptor) Outgoing(viewID string) []Edge {
	var result []Edge
	for _, edge := range descriptor.Edges {
		if edge.SourceViewID == viewID && edge.Navigable {
			result = append(result, edge)
		}
	}
	return result
}
