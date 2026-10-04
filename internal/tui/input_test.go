package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// typeKeys feeds keys to a focused input: a string is typed rune by rune, a
// tea.KeyType is pressed, and a tea.KeyMsg is sent as is.
func typeKeys(m textInput, keys ...any) textInput {
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			for _, r := range k {
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
		case tea.KeyType:
			m, _ = m.Update(tea.KeyMsg{Type: k})
		case tea.KeyMsg:
			m, _ = m.Update(k)
		}
	}
	return m
}

func focusedInput(limit, width int) textInput {
	m := newTextInput("Search...", limit, width)
	m.Focus()
	return m
}

func TestTextInputEditing(t *testing.T) {
	alt := func(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true} }
	tests := []struct {
		name    string
		keys    []any
		want    string
		wantPos int
	}{
		{"typing", []any{"nginx"}, "nginx", 5},
		{"backspace", []any{"nginx", tea.KeyBackspace, tea.KeyCtrlH}, "ngi", 3},
		{"backspace at start", []any{"ab", tea.KeyHome, tea.KeyBackspace}, "ab", 0},
		{"insert mid-word", []any{"ngnx", tea.KeyLeft, tea.KeyLeft, "i"}, "nginx", 3},
		{"delete forward", []any{"nginx", tea.KeyHome, tea.KeyDelete, tea.KeyCtrlD}, "inx", 0},
		{"home and end", []any{"web", tea.KeyCtrlA, "[", tea.KeyCtrlE, "]"}, "[web]", 5},
		{"delete word back", []any{"foo bar  ", tea.KeyCtrlW}, "foo ", 4},
		{"delete word back (alt)", []any{"foo bar", tea.KeyMsg{Type: tea.KeyBackspace, Alt: true}}, "foo ", 4},
		{"delete word forward", []any{"foo bar baz", tea.KeyHome, alt('d')}, " bar baz", 0},
		{"delete word forward on the last character", []any{"foo", tea.KeyLeft, alt('d')}, "fo", 2},
		{"delete word forward at the end", []any{"foo", alt('d')}, "foo", 3},
		{"word back and forward", []any{"one two three", alt('b'), alt('b'), "X", alt('f'), "Y"}, "one XtwoY three", 9},
		{"ctrl+left/right", []any{"one two", tea.KeyCtrlLeft, "_", tea.KeyCtrlRight, "!"}, "one _two!", 9},
		{"kill to end", []any{"nginx -g", tea.KeyLeft, tea.KeyLeft, tea.KeyLeft, tea.KeyCtrlK}, "nginx", 5},
		{"kill to start", []any{"nginx -g", tea.KeyLeft, tea.KeyLeft, tea.KeyCtrlU}, "-g", 0},
		{"keys without text type nothing", []any{"a", tea.KeyTab, tea.KeyCtrlV, tea.KeyF5, tea.KeyUp}, "a", 1},
		{"paste is one line", []any{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\tb\r\nc\x01d"), Paste: true}}, "a b  cd", 7},
		{"wide characters", []any{"世界", tea.KeyLeft, "x"}, "世x界", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := typeKeys(focusedInput(0, 50), tt.keys...)
			if m.Value() != tt.want || m.pos != tt.wantPos {
				t.Errorf("value %q, cursor %d; want %q, %d", m.Value(), m.pos, tt.want, tt.wantPos)
			}
		})
	}
}

func TestTextInputLimitsAndFocus(t *testing.T) {
	m := typeKeys(focusedInput(4, 8), "-12345")
	if m.Value() != "-123" {
		t.Errorf("char limit: value %q, want \"-123\"", m.Value())
	}
	m.SetValue("1\t2\n3456")
	if m.Value() != "1 2 " {
		t.Errorf("SetValue sanitizes and limits: %q", m.Value())
	}

	blurred := newTextInput("Search...", 0, 50)
	if got, _ := blurred.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}); got.Value() != "" {
		t.Error("an unfocused input took a key")
	}
	blurred.SetValue("seeded")
	if blurred.Value() != "seeded" || blurred.pos != 6 {
		t.Errorf("SetValue on an empty input: %q, cursor %d", blurred.Value(), blurred.pos)
	}
}

// Text wider than the box scrolls so the cursor stays visible.
func TestTextInputScrolls(t *testing.T) {
	m := typeKeys(focusedInput(0, 5), "abcdefghij")
	if got := string(m.value[m.offset:m.offsetRight]); got != "fghij" {
		t.Errorf("at the end, visible %q, want \"fghij\"", got)
	}
	m = typeKeys(m, tea.KeyHome)
	if m.offset != 0 || !strings.HasPrefix(string(m.value[m.offset:m.offsetRight]), "abcde") {
		t.Errorf("at the start, visible %q", string(m.value[m.offset:m.offsetRight]))
	}
	m = typeKeys(m, tea.KeyEnd, tea.KeyCtrlU)
	if m.offset != 0 || m.offsetRight != 0 {
		t.Errorf("after clearing, window [%d:%d]", m.offset, m.offsetRight)
	}
}

func TestTextInputView(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)
	reverse := lipgloss.NewStyle().Reverse(true).Render

	m := textInput{Prompt: "> ", Placeholder: "Search", Width: 10}
	if got, want := m.View(), "> Search  "; got != want {
		t.Errorf("blurred placeholder = %q, want %q", got, want)
	}
	m.Focus()
	if got, want := m.View(), "> "+reverse("S")+"earch  "; got != want {
		t.Errorf("focused placeholder = %q, want %q", got, want)
	}
	m = typeKeys(m, "nginx", tea.KeyLeft)
	if got, want := m.View(), "> ngin"+reverse("x")+"      "; got != want {
		t.Errorf("cursor on a character = %q, want %q", got, want)
	}
	m = typeKeys(m, tea.KeyEnd)
	if got, want := m.View(), "> nginx"+reverse(" ")+"     "; got != want {
		t.Errorf("cursor at the end = %q, want %q", got, want)
	}
	m.Blur()
	if got, want := m.View(), "> nginx      "; got != want {
		t.Errorf("blurred value = %q, want %q", got, want)
	}
}
