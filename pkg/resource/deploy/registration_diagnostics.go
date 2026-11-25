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

package deploy

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type registrationFrame struct {
	filename string
	display  string
	line     int
	snippet  bool
}

// ResourceSourceContext returns a resource's registration locations and their rendered presentation.
func ResourceSourceContext(deployment *Deployment, state *pkgresource.State) *diag.SourceContext {
	if state == nil {
		return nil
	}
	root := ""
	if deployment != nil {
		if source, ok := deployment.source.(*evalSource); ok {
			root = source.runinfo.ProjectRoot
		}
	}
	return newSourceContext(root, state.SourcePosition, state.StackTrace)
}

func newSourceContext(projectRoot, sourcePosition string, frames []pkgresource.StackFrame) *diag.SourceContext {
	if sourcePosition == "" && len(frames) == 0 {
		return nil
	}
	result := &diag.SourceContext{
		SourcePosition: sourcePosition,
		StackTrace:     append([]apitype.StackFrameV1(nil), frames...),
	}
	if projectRoot != "" {
		renderedFrames := frames
		if len(renderedFrames) == 0 && sourcePosition != "" {
			renderedFrames = []pkgresource.StackFrame{{SourcePosition: sourcePosition}}
		}
		result.Rendered = formatRegistrationFrames(projectRoot, renderedFrames)
	}
	return result
}

func formatRegistrationFrames(projectRoot string, frames []pkgresource.StackFrame) string {
	// Find the program capture regardless of the order in which captures were stitched.
	start, end, firstBoundary := 0, len(frames), len(frames)
	foundProgram := false
	for i, frame := range frames {
		if frame.Boundary == nil {
			continue
		}
		firstBoundary = min(firstBoundary, i)
		if foundProgram {
			end = i
			break
		}
		if frame.Boundary.Kind == apitype.StackTraceBoundaryKindProgram {
			start = i
			foundProgram = true
		}
	}
	if foundProgram {
		frames = frames[start:end]
	} else {
		frames = frames[:firstBoundary]
	}
	if len(frames) > 0 && frames[0].Boundary != nil {
		callSite := frames[0].Boundary.CallSite
		if callSite != "" {
			for i, frame := range frames {
				if frame.SourcePosition == callSite {
					frames = frames[i:]
					break
				}
			}
		}
	}
	var selected []registrationFrame
	for _, frame := range frames {
		if frame.Origin == "pulumi" || frame.Origin == "runtime" {
			continue
		}
		filename, line := "", 0
		position, err := url.Parse(frame.SourcePosition)
		if err == nil && position.Host == "" {
			line, _ = strconv.Atoi(strings.Split(position.Fragment, ",")[0])
			if line > 0 {
				switch position.Scheme {
				case "project":
					filename = filepath.Join(projectRoot, filepath.FromSlash(strings.TrimPrefix(position.Path, "/")))
				case "file":
					filename = filepath.FromSlash(position.Path)
				}
			}
		}
		if filename == "" {
			if frame.Module != "" {
				selected = append(selected, registrationFrame{display: frame.Module})
			}
			continue
		}
		rel, err := filepath.Rel(projectRoot, filename)
		local := err == nil && filepath.IsLocal(rel)
		display := filepath.ToSlash(filename)
		if local {
			display = filepath.ToSlash(rel)
		} else if frame.Module != "" {
			display = frame.Module + " — " + display
		}
		selected = append(selected, registrationFrame{
			filename: filename, display: display, line: line, snippet: local,
		})
	}
	slices.Reverse(selected)
	if len(selected) == 0 {
		return ""
	}
	var result strings.Builder
	result.WriteString(colors.Reset)
	width := len(strconv.Itoa(selected[len(selected)-1].line + 1))
	gutter := strings.Repeat(" ", width+1)
	filenameIndent := gutter + "   "
	locationWidth := 0
	for _, frame := range selected[:len(selected)-1] {
		location := frame.location()
		locationWidth = max(locationWidth, colors.MeasureColorizedString(location))
	}
	for i, frame := range selected {
		filename, line := frame.filename, frame.line
		location := frame.location()
		if i == len(selected)-1 && frame.snippet {
			result.WriteString(colors.BrightBlack + gutter + "┌─ " + colors.Reset)
			result.WriteString(colors.Cyan + colors.Bold + location + colors.Reset + "\n")
			start := max(1, line-1)
			source := registrationSourceLines(projectRoot, filename, start, line+1)
			if line-start >= len(source) {
				source = nil
			}
			highlighted := highlightRegistrationSource(filename, strings.Join(source, "\n"))
			for i, text := range source {
				number := start + i
				marker := " "
				style := colors.BrightBlack
				if number == line {
					marker = ">"
					style = colors.Cyan + colors.Bold
				}
				if i < len(highlighted) {
					text = highlighted[i]
				}
				fmt.Fprintf(&result, "%s%s%*d│ %s%s\n", style, marker, width, number, colors.Reset, text)
			}
			result.WriteString(colors.BrightBlack + gutter + "└─" + colors.Reset + "\n")
		} else {
			fmt.Fprintf(&result, "%s%s%s%s", filenameIndent, colors.Cyan, location, colors.Reset)
			text := ""
			if frame.snippet {
				text = literalRegistrationText(registrationSourceLine(projectRoot, filename, line))
			}
			if text != "" {
				padding := strings.Repeat(" ", locationWidth-colors.MeasureColorizedString(location)+2)
				fmt.Fprintf(&result, "%s%s%s%s", padding, colors.BrightBlack, text, colors.Reset)
			}
			result.WriteString("\n")
		}
	}
	return strings.TrimSuffix(result.String(), "\n")
}

func (frame registrationFrame) location() string {
	name := literalRegistrationText(logging.FilterString(frame.display))
	if frame.filename == "" {
		return name
	}
	return fmt.Sprintf("%s:%d", name, frame.line)
}

func registrationSourceLine(root, filename string, line int) string {
	lines := registrationSourceLines(root, filename, line, line)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// Split literal markup openers with a reset so colorization prints the original text.
func literalRegistrationText(s string) string {
	return strings.ReplaceAll(s, "<{%", "<"+colors.Reset+"{%")
}

func registrationSourceLines(root, filename string, first, last int) []string {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return nil
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 1024*1024))
	var lines []string
	for n := 1; n <= last && scanner.Scan(); n++ {
		if n >= first {
			lines = append(lines, scanner.Text())
		}
	}
	filtered := strings.Split(logging.FilterString(strings.Join(lines, "\n")), "\n")
	if len(filtered) != len(lines) {
		// A multiline redaction changed the line numbers, or there was no source.
		return nil
	}
	return filtered
}

func highlightRegistrationSource(filename, source string) []string {
	// A lexer can split a markup opener across tokens. Keep these snippets literal.
	if strings.Contains(source, "<{%") {
		return strings.Split(literalRegistrationText(source), "\n")
	}
	plain := strings.Split(source, "\n")
	lexer := lexers.Match(filename)
	if lexer == nil {
		return plain
	}
	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return plain
	}
	var output strings.Builder
	for token := iterator(); token != chroma.EOF; token = iterator() {
		style := ""
		switch {
		case token.Type.InCategory(chroma.Keyword):
			style = colors.Magenta
		case token.Type.InSubCategory(chroma.LiteralString):
			style = colors.Green
		case token.Type.InSubCategory(chroma.LiteralNumber):
			style = colors.Cyan
		case token.Type.InCategory(chroma.Comment):
			style = colors.BrightBlack
		case token.Type == chroma.NameBuiltin || token.Type == chroma.NameClass:
			style = colors.Cyan
		}
		for i, part := range strings.Split(token.Value, "\n") {
			if i > 0 {
				output.WriteString("\n")
			}
			if style != "" && part != "" {
				output.WriteString(style + part + colors.Reset)
			} else {
				output.WriteString(part)
			}
		}
	}
	return strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
}
