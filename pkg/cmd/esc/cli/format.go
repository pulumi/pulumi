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

// validEncodings lists the encodings each object may pair with. `value` has no dotenv/shell (those encode
// only a flat string map), and `process` has no bare string.
var validEncodings = map[outputObject]map[outputEncoding]bool{
	objectValue: {
		encodingString:       true,
		encodingJSON:         true,
		encodingYAML:         true,
		encodingJSONDetailed: true,
	},
	objectProcess: {
		encodingJSON:         true,
		encodingYAML:         true,
		encodingJSONDetailed: true,
		encodingDotenv:       true,
		encodingShell:        true,
	},
}

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

// formatFlagHelp renders the flag help listing every accepted format value: the legacy aliases plus every
// valid object:encoding pair, sorted.
func formatFlagHelp(prefix string) string {
	var values []string
	for alias := range formatAliases {
		values = append(values, alias)
	}
	for objectStr, object := range objectTokens {
		for encodingStr, encoding := range encodingTokens {
			if validEncodings[object][encoding] {
				values = append(values, objectStr+":"+encodingStr)
			}
		}
	}
	sort.Strings(values)
	return prefix + strings.Join(values, ", ")
}
