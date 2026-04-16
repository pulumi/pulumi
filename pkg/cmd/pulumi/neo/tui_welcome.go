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
	magenta := lipgloss.Color("5")
	dim := lipgloss.NewStyle().Faint(true)

	displayDir := w.workDir
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, w.workDir); err == nil && !strings.HasPrefix(rel, "..") {
			displayDir = "~/" + rel
		}
	}

	// Total box width (incl. border) = termWidth - 2 outer indent.
	// contentWidth = total - 2 border - 4 horizontal padding.
	totalWidth := max(w.termWidth, 30)
	contentWidth := max(totalWidth-2-2-4, 20)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(magenta).
		Width(contentWidth).
		Align(lipgloss.Center)

	infoText := displayDir
	if w.org != "" {
		infoText += " · org: " + w.org
	}
	if lipgloss.Width(infoText) > contentWidth && w.org != "" {
		orgSuffix := " · org: " + w.org
		maxPath := contentWidth - lipgloss.Width(orgSuffix)
		if maxPath > 3 {
			pathRunes := []rune(displayDir)
			if len(pathRunes) > maxPath {
				displayDir = string(pathRunes[:maxPath-3]) + "..."
			}
			infoText = displayDir + orgSuffix
		}
	}

	parts := []string{
		titleStyle.Render("Pulumi Neo"),
		"",
		w.greeting,
		"",
		strings.TrimRight(pulumipusArt, "\n"),
		"",
		dim.Render(infoText),
	}

	if w.consoleURL != "" {
		linkText := w.consoleURL
		maxLink := contentWidth - 2 // "⟡ " prefix
		if len([]rune(linkText)) > maxLink && maxLink > 3 {
			linkText = string([]rune(linkText)[:maxLink-3]) + "..."
		}
		hyperlink := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", w.consoleURL, linkText)
		parts = append(parts, dim.Render("⟡ "+hyperlink))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(magenta).
		Padding(1, 2).
		Margin(1, 0, 1, 2).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}
