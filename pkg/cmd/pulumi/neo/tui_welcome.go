// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package neo

import (
	_ "embed"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

//go:embed pulumipus.ans
var pulumipusArt string

// welcomeModel holds the data needed to render the startup banner.
type welcomeModel struct {
	org        string
	workDir    string
	username   string
	consoleURL string
	termWidth  int
	greeting   string // cached greeting, picked once at creation
}

// greetingTemplates maps time-of-day buckets to greeting format strings.
var greetingTemplates = map[string][]string{
	"morning": {
		"Morning, %s. What are we working on?",
		"Good morning, %s. What can I build for you?",
		"Morning, %s. Ready to ship something?",
		"Rise and ship, %s. What are we building?",
	},
	"afternoon": {
		"Afternoon, %s. What can I help with?",
		"Good afternoon, %s. What are we building?",
		"Hey %s, what can I help you with?",
		"Afternoon, %s. What should we work on?",
	},
	"evening": {
		"Evening, %s. What can I help with?",
		"Good evening, %s. What are we working on?",
		"Evening, %s. What should we build?",
		"Hey %s, what can I help with tonight?",
	},
	"night": {
		"Late one, %s? What can I help with?",
		"Burning the midnight oil, %s? What are we building?",
		"Night owl mode, %s. What can I help with?",
		"Up late, %s? Let's build something.",
	},
}

func timeOfDayKey(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

func pickGreeting(name string) string {
	if name == "" {
		return "What do you want to build today?"
	}
	key := timeOfDayKey(time.Now().Hour())
	templates := greetingTemplates[key]
	boldName := lipgloss.NewStyle().Bold(true).Render(name)
	return fmt.Sprintf(templates[rand.Intn(len(templates))], boldName) //nolint:gosec
}

// View renders the welcome box with the Pulumipus mascot art.
func (w welcomeModel) View() string {
	magenta := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	dim := lipgloss.NewStyle().Faint(true)
	bold := lipgloss.NewStyle().Bold(true)

	// Shorten home directory to ~ for display.
	displayDir := w.workDir
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, w.workDir); err == nil && !strings.HasPrefix(rel, "..") {
			displayDir = "~/" + rel
		}
	}

	artLines := strings.Split(strings.TrimRight(pulumipusArt, "\n"), "\n")

	// Box dimensions: 2-space indent + border chars + inner content.
	boxWidth := w.termWidth
	if boxWidth < 30 {
		boxWidth = 30
	}
	boxInner := boxWidth - 4 // 2-space indent + left border + right border
	if boxInner < 20 {
		boxInner = 20
	}

	var sb strings.Builder

	// Top border: ╭──── Pulumi Neo ────────────╮
	title := " Pulumi Neo "
	titleLen := len([]rune(title))
	leftDash := 4
	rightDash := boxInner - leftDash - titleLen
	if rightDash < 1 {
		rightDash = 1
	}
	sb.WriteString(fmt.Sprintf("\n  %s%s%s%s%s\n",
		magenta.Render("╭"), magenta.Render(strings.Repeat("─", leftDash)),
		bold.Foreground(lipgloss.Color("5")).Render(title),
		magenta.Render(strings.Repeat("─", rightDash)), magenta.Render("╮")))

	// Helper to write a box line with padding.
	boxLine := func(content string, visWidth int) {
		pad := boxInner - visWidth
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s%s\n",
			magenta.Render("│"), content, strings.Repeat(" ", pad), magenta.Render("│")))
	}

	// Blank line.
	boxLine("", 0)

	// Greeting (cached at model creation to avoid changing on every View call).
	greetContent := "  " + w.greeting
	boxLine(greetContent, lipgloss.Width(greetContent))

	// Blank line.
	boxLine("", 0)

	// Art lines.
	artIndent := 4
	for _, line := range artLines {
		vis := lipgloss.Width(line)
		content := strings.Repeat(" ", artIndent) + line
		boxLine(content, artIndent+vis)
	}

	// Blank line.
	boxLine("", 0)

	// Info line: path . org
	infoText := displayDir
	if w.org != "" {
		infoText += " · org: " + w.org
	}
	// Truncate if too long.
	maxInfo := boxInner - 4
	if len([]rune(infoText)) > maxInfo && w.org != "" {
		orgSuffix := " · org: " + w.org
		maxPath := maxInfo - len([]rune(orgSuffix))
		if maxPath > 3 {
			pathRunes := []rune(displayDir)
			if len(pathRunes) > maxPath {
				displayDir = string(pathRunes[:maxPath-3]) + "..."
			}
			infoText = displayDir + orgSuffix
		}
	}
	infoContent := "  " + dim.Render(infoText)
	boxLine(infoContent, lipgloss.Width(infoContent))

	// Session link (OSC 8 hyperlink).
	if w.consoleURL != "" {
		prefix := "  "
		linkText := w.consoleURL
		maxLink := boxInner - 4 - 2 // 2 for prefix
		if len([]rune(linkText)) > maxLink {
			linkText = string([]rune(linkText)[:maxLink-3]) + "..."
		}
		hyperlink := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", w.consoleURL, linkText)
		linkContent := prefix + dim.Render("⟡ "+hyperlink)
		boxLine(linkContent, lipgloss.Width(prefix+"⟡ "+linkText))
	}

	// Blank line.
	boxLine("", 0)

	// Bottom border.
	sb.WriteString(fmt.Sprintf("  %s%s%s\n",
		magenta.Render("╰"), magenta.Render(strings.Repeat("─", boxInner)), magenta.Render("╯")))

	return sb.String()
}
