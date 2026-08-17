package textinput

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func Test_CurrentSuggestion(t *testing.T) {
	textinput := New()
	textinput.ShowSuggestions = true

	suggestion := textinput.CurrentSuggestion()
	expected := ""
	if suggestion != expected {
		t.Fatalf("Error: expected no current suggestion but was %s", suggestion)
	}

	textinput.SetSuggestions([]string{"test1", "test2", "test3"})
	suggestion = textinput.CurrentSuggestion()
	expected = ""
	if suggestion != expected {
		t.Fatalf("Error: expected no current suggestion but was %s", suggestion)
	}

	textinput.SetValue("test")
	textinput.updateSuggestions()
	textinput.nextSuggestion()
	suggestion = textinput.CurrentSuggestion()
	expected = "test2"
	if suggestion != expected {
		t.Fatalf("Error: expected first suggestion but was %s", suggestion)
	}

	textinput.Blur()
	if strings.HasSuffix(textinput.View(), "test2") {
		t.Fatalf("Error: suggestions should not be rendered when input isn't focused. expected \"> test\" but got \"%s\"", textinput.View())
	}
}

func Test_SlicingOutsideCap(t *testing.T) {
	textinput := New()
	textinput.Placeholder = "作業ディレクトリを指定してください"
	textinput.Width = 32
	textinput.View()
}

func TestChinesePlaceholder(t *testing.T) {
	textinput := New()
	textinput.Placeholder = "输入消息..."
	textinput.Width = 20

	got := textinput.View()
	expected := "> 输入消息...       "
	if got != expected {
		t.Fatalf("expected %q but got %q", expected, got)
	}
}

func TestPlaceholderTruncate(t *testing.T) {
	textinput := New()
	textinput.Placeholder = "A very long placeholder, or maybe not so much"
	textinput.Width = 10

	got := textinput.View()
	expected := "> A very …"
	if got != expected {
		t.Fatalf("expected %q but got %q", expected, got)
	}
}

func ExampleValidateFunc() {
	creditCardNumber := New()
	creditCardNumber.Placeholder = "4505 **** **** 1234"
	creditCardNumber.Focus()
	creditCardNumber.CharLimit = 20
	creditCardNumber.Width = 30
	creditCardNumber.Prompt = ""
	// This anonymous function is a valid function for ValidateFunc.
	creditCardNumber.Validate = func(s string) error {
		// Credit Card Number should a string less than 20 digits
		// It should include 16 integers and 3 spaces
		if len(s) > 16+3 {
			return fmt.Errorf("CCN is too long")
		}

		if len(s) == 0 || len(s)%5 != 0 && (s[len(s)-1] < '0' || s[len(s)-1] > '9') {
			return fmt.Errorf("CCN is invalid")
		}

		// The last digit should be a number unless it is a multiple of 4 in which
		// case it should be a space
		if len(s)%5 == 0 && s[len(s)-1] != ' ' {
			return fmt.Errorf("CCN must separate groups with spaces")
		}

		// The remaining digits should be integers
		c := strings.ReplaceAll(s, " ", "")
		_, err := strconv.ParseInt(c, 10, 64)

		return err
	}
}

func keyPress(key rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}, Alt: false}
}

func sendString(m Model, str string) Model {
	for _, k := range str {
		m, _ = m.Update(keyPress(k))
	}

	return m
}
