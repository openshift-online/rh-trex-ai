package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type BindingID string

const (
	KeyQuit             BindingID = "quit"
	KeyForceQuit        BindingID = "force-quit"
	KeyHelp             BindingID = "help"
	KeyCommand          BindingID = "command"
	KeyFilter           BindingID = "filter"
	KeyCancel           BindingID = "cancel"
	KeySubmit           BindingID = "submit"
	KeyNextFocus        BindingID = "next-focus"
	KeyPreviousFocus    BindingID = "previous-focus"
	KeyNavigate         BindingID = "navigate"
	KeyDetail           BindingID = "detail"
	KeyRaw              BindingID = "raw"
	KeyActions          BindingID = "actions"
	KeyColumnsLeft      BindingID = "columns-left"
	KeyColumnsRight     BindingID = "columns-right"
	KeyChoicePrevious   BindingID = "choice-previous"
	KeyChoiceNext       BindingID = "choice-next"
	KeyScrollUp         BindingID = "scroll-up"
	KeyScrollDown       BindingID = "scroll-down"
	KeyPageUp           BindingID = "page-up"
	KeyPageDown         BindingID = "page-down"
	KeyScrollHome       BindingID = "scroll-home"
	KeyScrollEnd        BindingID = "scroll-end"
	KeyDismissAlert     BindingID = "dismiss-alert"
	KeyHistoryPrevious  BindingID = "history-previous"
	KeyHistoryNext      BindingID = "history-next"
	KeySuggestionNext   BindingID = "suggestion-next"
	KeySuggestionPrev   BindingID = "suggestion-previous"
	KeyAcceptSuggestion BindingID = "accept-suggestion"
	KeyToggleAutoscroll BindingID = "toggle-autoscroll"
	KeyAlertDetails     BindingID = "alert-details"
	KeySortNext         BindingID = "sort-next"
	KeySortDirection    BindingID = "sort-direction"
)

type BindingSpec struct {
	ID       BindingID
	Binding  key.Binding
	Global   bool
	Order    int
	Priority int
}

// ShortcutHint is the shared semantic representation used by dispatch-adjacent
// chrome and help. Presentation components must not reconstruct key labels.
type ShortcutHint struct {
	ID          BindingID
	Key         string
	Description string
	Order       int
	Priority    int
}

func (hint ShortcutHint) Text() string {
	return "<" + hint.Key + "> " + hint.Description
}

type KeyRegistry struct{ bindings map[BindingID]BindingSpec }

func DefaultKeyRegistry() KeyRegistry {
	specs := []BindingSpec{
		{KeyQuit, key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")), true, 10, 40},
		{KeyForceQuit, key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")), true, 11, 90},
		{KeyHelp, key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")), true, 20, 100},
		{KeyCommand, key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "resources")), true, 30, 80},
		{KeyFilter, key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")), true, 40, 80},
		{KeyCancel, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/cancel")), true, 50, 90},
		{KeySubmit, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/submit")), true, 60, 90},
		{KeyNextFocus, key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")), true, 70, 70},
		{KeyPreviousFocus, key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field")), true, 80, 60},
		{KeyNavigate, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "navigate")), false, 90, 75},
		{KeyDetail, key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "detail")), false, 100, 70},
		{KeyRaw, key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "raw")), false, 105, 70},
		{KeyActions, key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "actions")), false, 110, 70},
		{KeyColumnsLeft, key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "columns left")), false, 120, 65},
		{KeyColumnsRight, key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "columns right")), false, 130, 65},
		{KeyChoicePrevious, key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "previous choice")), false, 131, 70},
		{KeyChoiceNext, key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next choice")), false, 132, 70},
		{KeyScrollUp, key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "scroll up")), false, 133, 70},
		{KeyScrollDown, key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "scroll down")), false, 134, 70},
		{KeyPageUp, key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")), false, 135, 60},
		{KeyPageDown, key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")), false, 136, 60},
		{KeyScrollHome, key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "first line")), false, 137, 55},
		{KeyScrollEnd, key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "last line")), false, 138, 55},
		{KeyDismissAlert, key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "dismiss alert")), true, 140, 85},
		{KeyHistoryPrevious, key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous history")), false, 150, 60},
		{KeyHistoryNext, key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next history")), false, 160, 60},
		{KeySuggestionNext, key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "next suggestion")), false, 161, 60},
		{KeySuggestionPrev, key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "previous suggestion")), false, 162, 60},
		{KeyAcceptSuggestion, key.NewBinding(key.WithKeys("tab", "right", "ctrl+f"), key.WithHelp("tab/→/ctrl+f", "complete")), false, 163, 65},
		{KeyToggleAutoscroll, key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle autoscroll")), false, 170, 60},
		{KeyAlertDetails, key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "alert details")), true, 180, 85},
		{KeySortNext, key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "next sort")), false, 190, 50},
		{KeySortDirection, key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "reverse sort")), false, 200, 50},
	}
	registry := KeyRegistry{bindings: make(map[BindingID]BindingSpec, len(specs))}
	for _, spec := range specs {
		registry.bindings[spec.ID] = spec
	}
	return registry
}

func (registry KeyRegistry) ConfigureCommandInput(input *textinput.Model) {
	if input == nil {
		return
	}
	if spec, present := registry.bindings[KeySuggestionNext]; present {
		input.KeyMap.NextSuggestion = spec.Binding
	}
	if spec, present := registry.bindings[KeySuggestionPrev]; present {
		input.KeyMap.PrevSuggestion = spec.Binding
	}
}

func (registry KeyRegistry) Matches(message tea.KeyMsg, id BindingID) bool {
	spec, present := registry.bindings[id]
	return present && key.Matches(message, spec.Binding)
}

func (registry KeyRegistry) Hint(id BindingID) (ShortcutHint, bool) {
	spec, present := registry.bindings[id]
	if !present {
		return ShortcutHint{}, false
	}
	help := spec.Binding.Help()
	if help.Key == "" || help.Desc == "" {
		return ShortcutHint{}, false
	}
	return ShortcutHint{
		ID: id, Key: SanitizeCell(help.Key), Description: SanitizeCell(help.Desc),
		Order: spec.Order, Priority: spec.Priority,
	}, true
}

func (registry KeyRegistry) Hints(ids ...BindingID) string {
	parts := make([]string, 0, len(ids))
	seen := make(map[BindingID]bool)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if hint, present := registry.Hint(id); present {
			parts = append(parts, fmt.Sprintf("[%s] %s", hint.Key, hint.Description))
		}
	}
	return strings.Join(parts, "  ")
}

// Shortcuts returns fixed and generated actions in one deterministic order.
// Both the header palette and the complete help dialog consume this result.
func (registry KeyRegistry) Shortcuts(ids []BindingID, actions []LocalAction) []ShortcutHint {
	result := make([]ShortcutHint, 0, len(ids)+len(actions))
	seen := make(map[BindingID]bool)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		hint, present := registry.Hint(id)
		if !present {
			continue
		}
		result = append(result, hint)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Order < result[j].Order })
	for index, action := range actions {
		if action.Hotkey == "" || action.Label == "" {
			continue
		}
		result = append(result, ShortcutHint{
			Key: SanitizeCell(action.Hotkey), Description: SanitizeCell(action.Label),
			Order: 1000 + index, Priority: 75,
		})
	}
	return result
}

func (registry KeyRegistry) ShortcutHelp(ids []BindingID, actions []LocalAction) string {
	shortcuts := registry.Shortcuts(ids, actions)
	parts := make([]string, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		parts = append(parts, shortcut.Text())
	}
	return strings.Join(parts, "\n")
}

func (registry KeyRegistry) ColumnHint() string {
	left := registry.bindings[KeyColumnsLeft].Binding.Help().Key
	right := registry.bindings[KeyColumnsRight].Binding.Help().Key
	return fmt.Sprintf("[%s/%s] columns", left, right)
}

func (registry KeyRegistry) Reserved(keyName string) bool {
	keyName = bubbleKeyName(keyName)
	for _, spec := range registry.bindings {
		for _, candidate := range spec.Binding.Keys() {
			if bubbleKeyName(candidate) == keyName {
				return true
			}
		}
	}
	return false
}

func (registry KeyRegistry) ActionMatches(message tea.KeyMsg, operation Operation) bool {
	return operation.Presentation.Hotkey != "" && message.String() == bubbleKeyName(operation.Presentation.Hotkey)
}

func bubbleKeyName(keyName string) string {
	if strings.HasPrefix(keyName, "ctrl-") {
		return "ctrl+" + strings.TrimPrefix(keyName, "ctrl-")
	}
	return keyName
}
