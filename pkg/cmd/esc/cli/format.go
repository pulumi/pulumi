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

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// outputObject is what an `env open`/`get`/`diff` command produces: either the resolved value tree,
// or the process-environment projection of it (string-valued environment variables plus files
// materialized to disk and exposed as path variables).
type outputObject int

const (
	objectValue outputObject = iota
	objectProcess
)

// outputEncoding is how an outputObject is serialized.
type outputEncoding int

const (
	encodingString outputEncoding = iota
	encodingJSON
	encodingYAML
	encodingJSONDetailed
	encodingDotenv
	encodingShell
)

// renderFormat is the fully-resolved `<object>:<encoding>` selection behind the `--format`/`--value` flag.
type renderFormat struct {
	object   outputObject
	encoding outputEncoding
}

// isProcess reports whether the format produces the process-environment projection rather than the value.
func (f renderFormat) isProcess() bool {
	return f.object == objectProcess
}

// String renders the format in its canonical `<object>:<encoding>` form.
func (f renderFormat) String() string {
	return objectNames[f.object] + ":" + encodingNames[f.encoding]
}

// formatAliases maps the legacy single-word `--format` values to their compositional equivalents. Every
// legacy value keeps working and renders identically.
var formatAliases = map[string]renderFormat{
	"json":     {objectValue, encodingJSON},
	"yaml":     {objectValue, encodingYAML},
	"string":   {objectValue, encodingString},
	"detailed": {objectValue, encodingJSONDetailed},
	"dotenv":   {objectProcess, encodingDotenv},
	"shell":    {objectProcess, encodingShell},
}

var objectTokens = map[string]outputObject{
	"value":   objectValue,
	"process": objectProcess,
}

var encodingTokens = map[string]outputEncoding{
	"string":        encodingString,
	"json":          encodingJSON,
	"yaml":          encodingYAML,
	"json-detailed": encodingJSONDetailed,
	"dotenv":        encodingDotenv,
	"shell":         encodingShell,
}

var objectNames = map[outputObject]string{
	objectValue:   "value",
	objectProcess: "process",
}

// objectDescriptions explains, for flag help, what each object is and how the two differ.
var objectDescriptions = map[outputObject]string{
	objectValue: "the resolved configuration value tree",
	objectProcess: "the environmentVariables and files reserved keys projected as the string variables a " +
		"process consumes; files are written to disk and each variable holds the file path",
}

// objectOrder is the order objects appear in flag help.
var objectOrder = []outputObject{objectValue, objectProcess}

var encodingNames = map[outputEncoding]string{
	encodingString:       "string",
	encodingJSON:         "json",
	encodingYAML:         "yaml",
	encodingJSONDetailed: "json-detailed",
	encodingDotenv:       "dotenv",
	encodingShell:        "shell",
}

// formatDescriptor is one valid `<object>:<encoding>` pair together with the one-line description shown in
// flag help.
type formatDescriptor struct {
	renderFormat
	description string
}

// formats is the canonical, display-ordered set of valid `<object>:<encoding>` pairs. It is the single source
// of truth for which pairs are legal (validEncodings is derived from it) and how each is described.
var formats = []formatDescriptor{
	{renderFormat{objectValue, encodingString}, "resolved value as a single flattened string"},
	{renderFormat{objectValue, encodingJSON}, "resolved value tree as JSON"},
	{renderFormat{objectValue, encodingYAML}, "resolved value tree as YAML"},
	{
		renderFormat{objectValue, encodingJSONDetailed},
		"resolved value tree as JSON, annotated with secret flags and source provenance",
	},
	{renderFormat{objectProcess, encodingJSON}, "process environment as a flat JSON object"},
	{renderFormat{objectProcess, encodingYAML}, "process environment as a flat YAML mapping"},
	{renderFormat{objectProcess, encodingJSONDetailed}, "process environment as JSON, with a per-variable secret flag"},
	{renderFormat{objectProcess, encodingDotenv}, "process environment as dotenv assignments"},
	{renderFormat{objectProcess, encodingShell}, "process environment as shell export statements"},
}

// validEncodings lists the encodings each object may pair with, derived from formats so the two cannot drift.
var validEncodings = func() map[outputObject]map[outputEncoding]bool {
	m := map[outputObject]map[outputEncoding]bool{}
	for _, f := range formats {
		if m[f.object] == nil {
			m[f.object] = map[outputEncoding]bool{}
		}
		m[f.object][f.encoding] = true
	}
	return m
}()

// parseFormat resolves a `--format`/`--value` string into a renderFormat. It accepts both the legacy
// single-word aliases and the compositional `<object>:<encoding>` form, and rejects illegal pairs such as
// `value:dotenv`.
func parseFormat(s string) (renderFormat, error) {
	if f, ok := formatAliases[s]; ok {
		return f, nil
	}

	objectStr, encodingStr, ok := strings.Cut(s, ":")
	if !ok {
		return renderFormat{}, fmt.Errorf("unknown output format %q", s)
	}

	object, ok := objectTokens[objectStr]
	if !ok {
		return renderFormat{}, fmt.Errorf("unknown output object %q in format %q", objectStr, s)
	}
	encoding, ok := encodingTokens[encodingStr]
	if !ok {
		return renderFormat{}, fmt.Errorf("unknown output encoding %q in format %q", encodingStr, s)
	}
	if !validEncodings[object][encoding] {
		return renderFormat{}, fmt.Errorf("output format %q is not a valid object:encoding pair", s)
	}
	return renderFormat{object: object, encoding: encoding}, nil
}

// validateFormat parses a format and enforces that a property path is only used against the value object;
// a path navigates the value tree and is meaningless against the process-environment projection.
func validateFormat(s string, path resource.PropertyPath) (renderFormat, error) {
	f, err := parseFormat(s)
	if err != nil {
		return renderFormat{}, err
	}
	if f.isProcess() && len(path) != 0 {
		return renderFormat{}, fmt.Errorf("output format %q may not be used with a property path", s)
	}
	return f, nil
}

// formatFlagHelp renders the flag help: an intro sentence, then a described list of every canonical
// `<object>:<encoding>` value, then the legacy single-word aliases that map onto them.
func formatFlagHelp(intro string) string {
	width := 0
	for _, f := range formats {
		if n := len(f.String()); n > width {
			width = n
		}
	}

	var b strings.Builder
	b.WriteString(intro)

	aliases := make([]string, 0, len(formatAliases))
	aliasWidth := 0
	for alias := range formatAliases {
		aliases = append(aliases, alias)
		if len(alias) > aliasWidth {
			aliasWidth = len(alias)
		}
	}
	sort.Strings(aliases)
	b.WriteString("\n\naliases:")
	for _, alias := range aliases {
		fmt.Fprintf(&b, "\n  %-*s  %s", aliasWidth, alias, formatAliases[alias].String())
	}

	for _, object := range objectOrder {
		fmt.Fprintf(&b, "\n\n%s: %s", objectNames[object], objectDescriptions[object])
		for _, f := range formats {
			if f.object == object {
				fmt.Fprintf(&b, "\n  %-*s  %s", width, f.String(), f.description)
			}
		}
	}

	return b.String()
}
