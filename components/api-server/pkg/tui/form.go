package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type formFieldDescriptor struct {
	Name, Location string
	Type, Format   string
	Required       bool
	Enum           []any
	Default        any
	Body           bool
	RawJSON        bool
	Parameter      *Parameter
}

type formField struct {
	descriptor formFieldDescriptor
	input      textinput.Model
	enumIndex  int
	err        string
}

type FormDialog struct {
	title        string
	fields       []formField
	focus        int
	base         RequestInput
	inFlight     bool
	bodyRequired bool
	keys         KeyRegistry
}

type FormEvent struct {
	Request   RequestInput
	Submitted bool
	Canceled  bool
	Invalid   bool
}

func NewFormDialog(operation Operation, values map[string]any, keys KeyRegistry) *FormDialog {
	form := &FormDialog{title: actionLabel(operation), base: RequestInput{Values: cloneBindings(values)}, keys: keys}
	parameters := append([]Parameter(nil), operation.Parameters...)
	sort.SliceStable(parameters, func(i, j int) bool {
		left, right := parameterOrder(parameters[i].In), parameterOrder(parameters[j].In)
		if left != right {
			return left < right
		}
		return parameters[i].Name < parameters[j].Name
	})
	for index := range parameters {
		parameter := parameters[index]
		if value, present := operationParameterValue(operation, parameter, values); present && strings.TrimSpace(scalarString(value)) != "" {
			continue
		}
		copy := parameter
		form.fields = append(form.fields, newFormField(formFieldDescriptor{
			Name:     parameter.Name,
			Location: parameter.In, Type: parameter.Type, Format: parameter.Format,
			Required: parameter.Required, Enum: append([]any(nil), parameter.Enum...), Default: parameter.Default,
			Parameter: &copy,
		}))
	}
	if operation.RequestBody != nil {
		form.bodyRequired = operation.RequestBody.Required
		if len(operation.RequestBody.Fields) == 0 {
			form.fields = append(form.fields, newFormField(formFieldDescriptor{
				Name: "body", Location: "body", Type: "object",
				Required: operation.RequestBody.Required, Body: true, RawJSON: true,
			}))
		} else {
			for _, field := range operation.RequestBody.Fields {
				if field.ReadOnly {
					continue
				}
				form.fields = append(form.fields, newFormField(formFieldDescriptor{
					Name: field.Name, Location: "body",
					Type: field.Type, Format: field.Format, Required: field.Required,
					Enum: append([]any(nil), field.Enum...), Default: field.Default, Body: true,
				}))
			}
		}
	}
	sort.SliceStable(form.fields, func(left, right int) bool {
		return form.fields[left].descriptor.Required && !form.fields[right].descriptor.Required
	})
	if len(form.fields) > 0 {
		form.focusField(0)
	}
	return form
}

func newFormField(descriptor formFieldDescriptor) formField {
	input := textinput.New()
	input.CharLimit = maxActionInputBytes
	input.Prompt = ""
	field := formField{descriptor: descriptor, input: input, enumIndex: -1}
	if descriptor.Default != nil {
		input.SetValue(scalarString(descriptor.Default))
	}
	if len(descriptor.Enum) > 0 {
		for index, candidate := range descriptor.Enum {
			if scalarString(candidate) == input.Value() {
				field.enumIndex = index
			}
		}
		if field.enumIndex < 0 && descriptor.Required && descriptor.Default == nil {
			field.enumIndex = 0
			input.SetValue(scalarString(descriptor.Enum[0]))
		}
	}
	field.input = input
	return field
}

func (form *FormDialog) Kind() DialogKind { return DialogForm }
func (form *FormDialog) Title() string    { return form.title }
func (form *FormDialog) Footer(theme Theme) string {
	keys := []BindingID{KeyNextFocus, KeyPreviousFocus}
	for index := range form.fields {
		if len(form.fields[index].descriptor.Enum) > 0 {
			keys = append(keys, KeyChoicePrevious, KeyChoiceNext)
			break
		}
	}
	keys = append(keys, KeyCancel, KeySubmit)
	parts := make([]string, 0, len(keys))
	for _, id := range keys {
		if hint, present := form.keys.Hint(id); present {
			parts = append(parts, theme.DialogAction(hint, id == KeySubmit))
		}
	}
	return strings.Join(parts, "  ")
}

func (form *FormDialog) Content(theme Theme) string {
	if len(form.fields) == 0 {
		return "No input is required."
	}
	nameWidth, typeWidth, requiredWidth := 0, 0, 0
	for index := range form.fields {
		field := &form.fields[index]
		nameWidth = max(nameWidth, ansi.StringWidth(SanitizeCell(field.descriptor.Name)))
		typeWidth = max(typeWidth, ansi.StringWidth(formFieldType(field.descriptor)))
		requiredWidth = max(requiredWidth, ansi.StringWidth(formFieldRequiredness(field.descriptor)))
	}
	inputOffset := 2 + nameWidth + 2 + typeWidth + 2 + requiredWidth + 2
	var lines []string
	for index := range form.fields {
		field := &form.fields[index]
		marker := "  "
		if index == form.focus {
			marker = "> "
		}
		required := formFieldRequiredness(field.descriptor)
		typeName := formFieldType(field.descriptor)
		value := field.input.View()
		if len(field.descriptor.Enum) > 0 {
			choice := field.input.Value()
			if choice == "" {
				choice = "unset"
			}
			value = "‹ " + SanitizeCell(choice) + " ›"
		}
		name := SanitizeCell(field.descriptor.Name)
		heading := theme.FieldTitle(name) + strings.Repeat(" ", max(0, nameWidth-ansi.StringWidth(name)))
		typeMetadata := theme.FieldMetadata(typeName) + strings.Repeat(" ", max(0, typeWidth-ansi.StringWidth(typeName)))
		requiredMetadata := theme.FieldMetadata(required) + strings.Repeat(" ", max(0, requiredWidth-ansi.StringWidth(required)))
		lines = append(lines, marker+heading+"  "+typeMetadata+"  "+requiredMetadata+": "+value)
		if field.err != "" {
			lines = append(lines, strings.Repeat(" ", inputOffset)+theme.FieldError("! "+SanitizeCell(field.err)))
		}
	}
	if form.inFlight {
		lines = append(lines, "", "Submitting…")
	}
	return strings.Join(lines, "\n")
}

func formFieldType(descriptor formFieldDescriptor) string {
	typeName := SanitizeCell(descriptor.Type)
	if descriptor.Format != "" {
		typeName += "/" + SanitizeCell(descriptor.Format)
	}
	return typeName
}

func formFieldRequiredness(descriptor formFieldDescriptor) string {
	if descriptor.Required {
		return "required"
	}
	return "optional"
}

func (form *FormDialog) Update(message tea.KeyMsg) (FormEvent, tea.Cmd) {
	if form.inFlight {
		return FormEvent{}, nil
	}
	if form.keys.Matches(message, KeyCancel) {
		return FormEvent{Canceled: true}, nil
	}
	if len(form.fields) == 0 {
		if form.keys.Matches(message, KeySubmit) {
			form.inFlight = true
			return FormEvent{Request: form.base, Submitted: true}, nil
		}
		return FormEvent{}, nil
	}
	if form.keys.Matches(message, KeyNextFocus) {
		form.focusField((form.focus + 1) % len(form.fields))
		return FormEvent{}, textinput.Blink
	}
	if form.keys.Matches(message, KeyPreviousFocus) {
		form.focusField((form.focus - 1 + len(form.fields)) % len(form.fields))
		return FormEvent{}, textinput.Blink
	}
	field := &form.fields[form.focus]
	if len(field.descriptor.Enum) > 0 && (form.keys.Matches(message, KeyChoicePrevious) || form.keys.Matches(message, KeyChoiceNext)) {
		direction := 1
		if form.keys.Matches(message, KeyChoicePrevious) {
			direction = -1
		}
		allowUnset := !field.descriptor.Required && field.descriptor.Default == nil
		if allowUnset {
			position := field.enumIndex + 1
			position = (position + direction + len(field.descriptor.Enum) + 1) % (len(field.descriptor.Enum) + 1)
			field.enumIndex = position - 1
			if field.enumIndex < 0 {
				field.input.SetValue("")
			} else {
				field.input.SetValue(scalarString(field.descriptor.Enum[field.enumIndex]))
			}
		} else {
			start := field.enumIndex
			if start < 0 {
				start = 0
			}
			field.enumIndex = (start + direction + len(field.descriptor.Enum)) % len(field.descriptor.Enum)
			field.input.SetValue(scalarString(field.descriptor.Enum[field.enumIndex]))
		}
		field.err = ""
		return FormEvent{}, nil
	}
	if form.keys.Matches(message, KeySubmit) {
		if err := validateFormField(field); err != nil {
			field.err = err.Error()
			return FormEvent{Invalid: true}, nil
		}
		field.err = ""
		if form.focus < len(form.fields)-1 {
			form.focusField(form.focus + 1)
			return FormEvent{}, textinput.Blink
		}
		request, valid := form.request()
		if !valid {
			return FormEvent{Invalid: true}, nil
		}
		form.inFlight = true
		return FormEvent{Request: request, Submitted: true}, nil
	}
	var command tea.Cmd
	field.input, command = field.input.Update(message)
	field.err = ""
	return FormEvent{}, command
}

func (form *FormDialog) request() (RequestInput, bool) {
	request := RequestInput{Values: cloneBindings(form.base.Values)}
	body := make(map[string]any)
	valid := true
	for index := range form.fields {
		field := &form.fields[index]
		if err := validateFormField(field); err != nil {
			field.err = err.Error()
			valid = false
			continue
		}
		value := strings.TrimSpace(field.input.Value())
		if value == "" {
			continue
		}
		if field.descriptor.RawJSON {
			request.Body = []byte(value)
			continue
		}
		parsed, err := parseFormValue(field.descriptor.Type, field.descriptor.Format, value)
		if err != nil {
			field.err = err.Error()
			valid = false
			continue
		}
		if field.descriptor.Body {
			body[field.descriptor.Name] = parsed
		} else if field.descriptor.Parameter != nil {
			parameterValue, parseErr := parseParameterInput(*field.descriptor.Parameter, value)
			if parseErr != nil {
				field.err = parseErr.Error()
				valid = false
				continue
			}
			request.Values[ParameterValueKey(field.descriptor.Location, field.descriptor.Name)] = parameterValue
		}
	}
	if len(body) > 0 || (form.bodyRequired && request.Body == nil) {
		data, err := json.Marshal(body)
		if err != nil {
			return request, false
		}
		request.Body = data
	}
	return request, valid
}

func validateFormField(field *formField) error {
	value := strings.TrimSpace(field.input.Value())
	if value == "" {
		if field.descriptor.Required {
			return fmt.Errorf("%s is required", field.descriptor.Name)
		}
		return nil
	}
	if len(field.descriptor.Enum) > 0 {
		matched := false
		for _, candidate := range field.descriptor.Enum {
			if value == scalarString(candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("must be one of the documented choices")
		}
	}
	if field.descriptor.RawJSON {
		decoder := json.NewDecoder(bytes.NewBufferString(value))
		decoder.UseNumber()
		var parsed any
		if err := decoder.Decode(&parsed); err != nil {
			return fmt.Errorf("must be valid JSON")
		}
		return ensureJSONEOF(decoder)
	}
	_, err := parseFormValue(field.descriptor.Type, field.descriptor.Format, value)
	if err != nil {
		return err
	}
	if field.descriptor.Parameter != nil {
		_, err = parseParameterInput(*field.descriptor.Parameter, value)
	}
	return err
}

func parseFormValue(typeName, format, value string) (any, error) {
	switch typeName {
	case "boolean":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("must be true or false")
		}
		return parsed, nil
	case "integer":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("must be an integer")
		}
		return parsed, nil
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("must be a number")
		}
		return json.Number(value), nil
	case "array", "object":
		decoder := json.NewDecoder(bytes.NewBufferString(value))
		decoder.UseNumber()
		var parsed any
		if err := decoder.Decode(&parsed); err != nil {
			return nil, fmt.Errorf("must be valid JSON")
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		if format == "date" {
			if _, err := time.Parse(time.DateOnly, value); err != nil {
				return nil, fmt.Errorf("must be a date in YYYY-MM-DD form")
			}
		}
		if format == "date-time" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return nil, fmt.Errorf("must be an RFC 3339 date-time")
			}
		}
		return value, nil
	}
}

func (form *FormDialog) focusField(index int) {
	if len(form.fields) == 0 {
		return
	}
	for fieldIndex := range form.fields {
		form.fields[fieldIndex].input.Blur()
	}
	form.focus = max(0, min(index, len(form.fields)-1))
	form.fields[form.focus].input.Focus()
}

func parameterOrder(location string) int {
	switch location {
	case "path":
		return 0
	case "query":
		return 1
	case "header":
		return 2
	default:
		return 3
	}
}

type ConfirmationDialog struct {
	confirmation Confirmation
	label        string
	confirmFocus bool
	inFlight     bool
	keys         KeyRegistry
}

func NewConfirmationDialog(label string, confirmation Confirmation, keys KeyRegistry) *ConfirmationDialog {
	return &ConfirmationDialog{confirmation: confirmation, label: label, keys: keys}
}

func (dialog *ConfirmationDialog) Kind() DialogKind { return DialogConfirmation }
func (dialog *ConfirmationDialog) Title() string {
	if title := strings.TrimSpace(dialog.confirmation.Title); title != "" {
		return title
	}
	if dialog.confirmation.Destructive {
		return "Delete"
	}
	return "Confirm"
}
func (dialog *ConfirmationDialog) Footer(Theme) string { return "" }
func (dialog *ConfirmationDialog) Content(theme Theme) string {
	message := strings.TrimSpace(SanitizeCell(dialog.confirmation.Message))
	if message == "" {
		message = strings.TrimSpace(SanitizeCell(dialog.label)) + "?"
	}
	affirmative := "Confirm"
	if dialog.confirmation.Destructive {
		affirmative = "Delete"
	}
	if dialog.inFlight {
		affirmative = "Working…"
	}
	focus := 0
	if dialog.confirmFocus {
		focus = 1
	}
	buttons := renderDialogButtons(theme, "Cancel", affirmative, focus, 5)
	contentWidth := max(36, ansi.StringWidth(message), ansi.StringWidth(buttons))
	return "\n" + centerDialogLine(message, contentWidth) + "\n\n" + centerDialogLine(buttons, contentWidth) + "\n"
}

func (dialog *ConfirmationDialog) Update(message tea.KeyMsg) (confirmed, canceled bool) {
	if dialog.inFlight {
		return false, false
	}
	if dialog.keys.Matches(message, KeyCancel) {
		return false, true
	}
	if dialog.keys.Matches(message, KeyPreviousFocus) || dialog.keys.Matches(message, KeyChoicePrevious) {
		dialog.confirmFocus = false
		return false, false
	}
	if dialog.keys.Matches(message, KeyNextFocus) || dialog.keys.Matches(message, KeyChoiceNext) {
		dialog.confirmFocus = true
		return false, false
	}
	if dialog.keys.Matches(message, KeySubmit) {
		if !dialog.confirmFocus {
			return false, true
		}
		dialog.inFlight = true
		return true, false
	}
	return false, false
}

func centerDialogLine(value string, width int) string {
	valueWidth := ansi.StringWidth(value)
	left := max(0, (width-valueWidth)/2)
	right := max(0, width-valueWidth-left)
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func renderDialogButtons(theme Theme, left, right string, focus, gap int) string {
	return theme.DialogButton(left, focus == 0) + strings.Repeat(" ", max(1, gap)) + theme.DialogButton(right, focus == 1)
}
