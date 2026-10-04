package tui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// textInput is the single-line input behind the search boxes and the renice
// prompt: shell-style editing keys, a block cursor while focused, and a
// placeholder while empty. Text wider than Width scrolls horizontally.
//
// It replaces bubbles' textinput, whose clipboard dependency searches PATH for
// clipboard tools every time witr starts (most of a second on WSL, where PATH
// includes the Windows directories). It behaves as textinput did in witr: no
// suggestions, and no blinking or Ctrl+V paste, whose messages the TUI never
// routed back to the input. Terminals paste on their own.
type textInput struct {
	Prompt           string
	Placeholder      string
	PromptStyle      lipgloss.Style
	PlaceholderStyle lipgloss.Style
	CharLimit        int // maximum length in runes; 0 means none
	Width            int // visible columns; 0 means unlimited

	value []rune
	pos   int // cursor position in value
	// offset and offsetRight bound the visible part of value when it is wider
	// than Width.
	offset, offsetRight int
	focus               bool
}

// newTextInput returns an empty, unfocused input.
func newTextInput(placeholder string, charLimit, width int) textInput {
	return textInput{
		Prompt:           "> ",
		Placeholder:      placeholder,
		PromptStyle:      promptStyle,
		PlaceholderStyle: placeholderStyle,
		CharLimit:        charLimit,
		Width:            width,
	}
}

func (m textInput) Value() string { return string(m.value) }
func (m textInput) Focused() bool { return m.focus }
func (m *textInput) Focus()       { m.focus = true }
func (m *textInput) Blur()        { m.focus = false }

// SetValue replaces the text. The cursor moves to the end when the input was
// empty or the cursor would fall past the new text.
func (m *textInput) SetValue(s string) {
	m.setValue(sanitizeInput([]rune(s)))
}

func (m *textInput) setValue(runes []rune) {
	empty := len(m.value) == 0
	if m.CharLimit > 0 && len(runes) > m.CharLimit {
		runes = runes[:m.CharLimit]
	}
	m.value = runes
	if (m.pos == 0 && empty) || m.pos > len(m.value) {
		m.setCursor(len(m.value))
	}
	m.scroll()
}

func (m *textInput) setCursor(pos int) {
	m.pos = max(0, min(pos, len(m.value)))
	m.scroll()
}

// Update applies a key press to a focused input.
func (m textInput) Update(msg tea.Msg) (textInput, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !m.focus || !ok {
		return m, nil
	}
	switch key.String() {
	case "alt+backspace", "ctrl+w":
		m.deleteWordBackward()
	case "backspace", "ctrl+h":
		if len(m.value) > 0 {
			m.value = append(m.value[:max(0, m.pos-1)], m.value[m.pos:]...)
			if m.pos > 0 {
				m.setCursor(m.pos - 1)
			}
		}
	case "alt+left", "ctrl+left", "alt+b":
		m.wordBackward()
	case "left", "ctrl+b":
		if m.pos > 0 {
			m.setCursor(m.pos - 1)
		}
	case "alt+right", "ctrl+right", "alt+f":
		m.wordForward()
	case "right", "ctrl+f":
		if m.pos < len(m.value) {
			m.setCursor(m.pos + 1)
		}
	case "home", "ctrl+a":
		m.setCursor(0)
	case "delete", "ctrl+d":
		if m.pos < len(m.value) {
			m.value = append(m.value[:m.pos], m.value[m.pos+1:]...)
		}
	case "end", "ctrl+e":
		m.setCursor(len(m.value))
	case "ctrl+k":
		m.value = m.value[:m.pos]
		m.setCursor(len(m.value))
	case "ctrl+u":
		m.value = m.value[m.pos:]
		m.offset = 0
		m.setCursor(0)
	case "alt+delete", "alt+d":
		m.deleteWordForward()
	default:
		// Typed characters, and pasted text (bracketed paste).
		m.insert(key.Runes)
	}
	m.scroll()
	return m, nil
}

// sanitizeInput keeps the input on one line: tabs and line breaks become
// spaces, and other control characters and invalid runes are dropped.
func sanitizeInput(runes []rune) []rune {
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		switch {
		case r == '\t', r == '\r', r == '\n':
			out = append(out, ' ')
		case r == unicode.ReplacementChar, unicode.IsControl(r):
		default:
			out = append(out, r)
		}
	}
	return out
}

func (m *textInput) insert(runes []rune) {
	add := sanitizeInput(runes)
	if m.CharLimit > 0 {
		room := m.CharLimit - len(m.value)
		if room <= 0 {
			return
		}
		add = add[:min(len(add), room)]
	}
	value := make([]rune, 0, len(m.value)+len(add))
	value = append(value, m.value[:m.pos]...)
	value = append(value, add...)
	value = append(value, m.value[m.pos:]...)
	m.pos += len(add)
	m.setValue(value)
}

// scroll moves the visible window, when the text is wider than Width, so that
// it shows the cursor.
func (m *textInput) scroll() {
	if m.Width <= 0 || uniseg.StringWidth(string(m.value)) <= m.Width {
		m.offset = 0
		m.offsetRight = len(m.value)
		return
	}
	m.offsetRight = min(m.offsetRight, len(m.value))
	if m.pos < m.offset {
		m.offset = m.pos
		w, i := 0, 0
		runes := m.value[m.offset:]
		for i < len(runes) && w <= m.Width {
			w += runewidth.RuneWidth(runes[i])
			if w <= m.Width+1 {
				i++
			}
		}
		m.offsetRight = m.offset + i
	} else if m.pos >= m.offsetRight {
		m.offsetRight = m.pos
		w := 0
		runes := m.value[:m.offsetRight]
		i := len(runes) - 1
		for i > 0 && w < m.Width {
			w += runewidth.RuneWidth(runes[i])
			if w <= m.Width {
				i--
			}
		}
		m.offset = m.offsetRight - (len(runes) - 1 - i)
	}
}

// deleteWordBackward deletes from the start of the word before the cursor.
func (m *textInput) deleteWordBackward() {
	if m.pos == 0 || len(m.value) == 0 {
		return
	}
	oldPos := m.pos
	m.setCursor(m.pos - 1)
	for unicode.IsSpace(m.value[m.pos]) {
		if m.pos <= 0 {
			break
		}
		m.setCursor(m.pos - 1)
	}
	for m.pos > 0 {
		if !unicode.IsSpace(m.value[m.pos]) {
			m.setCursor(m.pos - 1)
		} else {
			m.setCursor(m.pos + 1) // keep the space before the word
			break
		}
	}
	if oldPos > len(m.value) {
		m.value = m.value[:m.pos]
	} else {
		m.value = append(m.value[:m.pos], m.value[oldPos:]...)
	}
}

// deleteWordForward deletes to the end of the word after the cursor.
func (m *textInput) deleteWordForward() {
	if m.pos >= len(m.value) || len(m.value) == 0 {
		return
	}
	oldPos := m.pos
	m.setCursor(m.pos + 1)
	for m.pos < len(m.value) && unicode.IsSpace(m.value[m.pos]) {
		m.setCursor(m.pos + 1)
	}
	for m.pos < len(m.value) && !unicode.IsSpace(m.value[m.pos]) {
		m.setCursor(m.pos + 1)
	}
	m.value = append(m.value[:oldPos], m.value[m.pos:]...)
	m.setCursor(oldPos)
}

// wordBackward moves the cursor to the start of the previous word.
func (m *textInput) wordBackward() {
	if m.pos == 0 || len(m.value) == 0 {
		return
	}
	i := m.pos - 1
	for i >= 0 && unicode.IsSpace(m.value[i]) {
		m.setCursor(m.pos - 1)
		i--
	}
	for i >= 0 && !unicode.IsSpace(m.value[i]) {
		m.setCursor(m.pos - 1)
		i--
	}
}

// wordForward moves the cursor to the end of the next word.
func (m *textInput) wordForward() {
	if m.pos >= len(m.value) || len(m.value) == 0 {
		return
	}
	i := m.pos
	for i < len(m.value) && unicode.IsSpace(m.value[i]) {
		m.setCursor(m.pos + 1)
		i++
	}
	for i < len(m.value) && !unicode.IsSpace(m.value[i]) {
		m.setCursor(m.pos + 1)
		i++
	}
}

// View renders the prompt and the visible text, with the cursor as a
// reverse-video cell while focused, padded to Width.
func (m textInput) View() string {
	if len(m.value) == 0 && m.Placeholder != "" {
		return m.placeholderView()
	}
	text := lipgloss.NewStyle().Inline(true).Render
	value := m.value[m.offset:m.offsetRight]
	pos := max(0, m.pos-m.offset)
	v := text(string(value[:pos]))
	if pos < len(value) {
		v += m.cursorView(string(value[pos]), lipgloss.NewStyle())
		v += text(string(value[pos+1:]))
	} else {
		v += m.cursorView(" ", lipgloss.NewStyle())
	}
	if valWidth := uniseg.StringWidth(string(value)); m.Width > 0 && valWidth <= m.Width {
		padding := max(0, m.Width-valWidth)
		if pos < len(value) {
			padding++
		}
		v += text(strings.Repeat(" ", padding))
	}
	return m.PromptStyle.Render(m.Prompt) + v
}

// cursorView renders the cell under the cursor: reversed while focused, in
// style otherwise.
func (m textInput) cursorView(char string, style lipgloss.Style) string {
	if m.focus {
		return lipgloss.NewStyle().Inline(true).Reverse(true).Render(char)
	}
	return style.Inline(true).Render(char)
}

func (m textInput) placeholderView() string {
	style := m.PlaceholderStyle.Inline(true).Render
	p := m.PromptStyle.Render(m.Prompt)
	first, rest, _, _ := uniseg.FirstGraphemeClusterInString(m.Placeholder, 0)
	v := m.cursorView(first, m.PlaceholderStyle)
	if m.Width < 1 && uniseg.StringWidth(rest) <= 1 {
		return p + v
	}
	if m.Width > 0 {
		width := m.Width - lipgloss.Width(p) - lipgloss.Width(v)
		rest = ansi.Truncate(rest, width, "…")
		v += style(rest) + strings.Repeat(" ", max(0, width-lipgloss.Width(rest)))
	} else {
		v += style(rest)
	}
	return p + v
}
