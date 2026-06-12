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
	"strings"

	"charm.land/lipgloss/v2"
)

// thinkingLabel is the label shown by the busy indicator while the agent is
// thinking (i.e. between tools, after a streaming message, etc.).
const thinkingLabel = "Thinking..."

// shimmerKind selects how a busy block's label is animated.
type shimmerKind int

const (
	// shimmerVerb pulses a magenta spotlight that orbits a short label
	// (e.g. "Thinking...").
	shimmerVerb shimmerKind = iota
	// shimmerWave sweeps a grayscale brightness ramp left-to-right across
	// a longer label (e.g. "read_file ...").
	shimmerWave
)

var (
	verbPeakStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	verbNearStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	verbDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Faint(true)

	// Wave palettes: a brightness ramp the wave sweeps across the label.
	// The dark-bg ramp uses near-white grays; the light-bg ramp inverts to
	// near-black grays so the wave stays visible on either background.
	waveStyles = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("253")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
	}
	waveStylesLight = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("235")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	}
	waveDimStyle = lipgloss.NewStyle().Faint(true)
)

// shimmerLabel renders text with the shimmer effect selected by kind. Frame
// is the animation tick counter; callers should pass a value that advances
// monotonically (modulo any safe bound) as the spinner ticks.
// hasDarkBackground picks the wave palette; the spotlight is magenta and
// reads on either background.
func shimmerLabel(text string, kind shimmerKind, frame int, hasDarkBackground bool) string {
	if text == "" {
		return ""
	}
	switch kind {
	case shimmerWave:
		return buildWave(text, frame, hasDarkBackground)
	case shimmerVerb:
		fallthrough
	default:
		return buildSpotlight(text, frame)
	}
}

// buildSpotlight renders text with a single bold-magenta peak character that
// orbits the string, with magenta neighbors and dim-magenta tail.
func buildSpotlight(text string, frame int) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	pos := frame % n
	var sb strings.Builder
	for i, r := range runes {
		diff := i - pos
		if diff < 0 {
			diff = -diff
		}
		if wrap := n - diff; wrap < diff {
			diff = wrap
		}
		ch := string(r)
		switch diff {
		case 0:
			sb.WriteString(verbPeakStyle.Render(ch))
		case 1:
			sb.WriteString(verbNearStyle.Render(ch))
		default:
			sb.WriteString(verbDimStyle.Render(ch))
		}
	}
	return sb.String()
}

// buildWave renders text with a grayscale brightness ramp that sweeps
// left-to-right across the string, then restarts after a brief lull where
// every character renders dim.
func buildWave(text string, frame int, hasDarkBackground bool) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	palette := waveStyles
	if !hasDarkBackground {
		palette = waveStylesLight
	}
	wavePos := frame % (len(runes) + len(palette))
	var sb strings.Builder
	for i, r := range runes {
		ch := string(r)
		dist := wavePos - i
		if dist >= 0 && dist < len(palette) {
			sb.WriteString(palette[dist].Render(ch))
		} else {
			sb.WriteString(waveDimStyle.Render(ch))
		}
	}
	return sb.String()
}
