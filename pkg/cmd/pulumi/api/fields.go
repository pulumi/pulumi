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

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// ParsedField is one -F / -f / --input value, already decoded into the
// right Go type for the target encoding (body JSON vs query string).
type ParsedField struct {
	Key   string
	Value any  // string, bool, float64, int64, nil
	Raw   bool // from -f: always emit as string
}

// parseField parses a single `key=value` spec into a ParsedField.
// typed=true uses gh-style type inference:
//   - "true"/"false" → bool
//   - "null" → nil
//   - integer literal → int64
//   - float literal → float64
//   - JSON object (`{...}`) or array (`[...]`) → parsed into map/slice so
//     callers can build nested request bodies inline
//   - "@path" → file contents as string (or stdin when path=="-")
//   - everything else → string
//
// typed=false always emits a string value (for `-f`).
func parseField(spec string, typed bool, stdin io.Reader) (ParsedField, error) {
	eq := strings.IndexByte(spec, '=')
	if eq < 0 {
		return ParsedField{}, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"field must be key=value: "+spec).WithField("field")
	}
	key, val := spec[:eq], spec[eq+1:]
	if key == "" {
		return ParsedField{}, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"field key must not be empty: "+spec).WithField("field")
	}
	// @file / @- reads (applies to both typed and raw).
	if strings.HasPrefix(val, "@") {
		source := val[1:]
		raw, err := readAtSource(source, stdin)
		if err != nil {
			return ParsedField{}, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				fmt.Sprintf("reading field %q: %v", key, err)).WithField("field")
		}
		return ParsedField{Key: key, Value: string(raw), Raw: !typed}, nil
	}
	if !typed {
		return ParsedField{Key: key, Value: val, Raw: true}, nil
	}
	// Type inference.
	switch val {
	case "true":
		return ParsedField{Key: key, Value: true}, nil
	case "false":
		return ParsedField{Key: key, Value: false}, nil
	case "null":
		return ParsedField{Key: key, Value: nil}, nil
	}
	// Numbers — only bare decimal, no hex/octal/exp to avoid surprises.
	if n, err := strconv.ParseInt(val, 10, 64); err == nil {
		return ParsedField{Key: key, Value: n}, nil
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return ParsedField{Key: key, Value: f}, nil
	}
	// JSON object/array literal: parse so nested bodies work without a file.
	// Only triggers on `{` or `[` prefixes — bare strings are never ambiguous.
	if len(val) > 0 && (val[0] == '{' || val[0] == '[') {
		var parsed any
		if err := json.Unmarshal([]byte(val), &parsed); err == nil {
			return ParsedField{Key: key, Value: parsed}, nil
		}
	}
	return ParsedField{Key: key, Value: val}, nil
}

// readAtSource implements `@path` and `@-` file-or-stdin reading.
func readAtSource(source string, stdin io.Reader) ([]byte, error) {
	if source == "-" {
		if stdin == nil {
			stdin = os.Stdin
		}
		return io.ReadAll(stdin)
	}
	return os.ReadFile(source)
}

// parseFields parses all -F / -f / --header values into structured slices.
// `fields` are typed, `rawFields` are strings. Duplicate keys are allowed;
// callers decide whether to combine or overwrite per encoding target.
func parseFields(typed, raw []string, stdin io.Reader) ([]ParsedField, error) {
	var out []ParsedField
	for _, spec := range typed {
		f, err := parseField(spec, true, stdin)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	for _, spec := range raw {
		f, err := parseField(spec, false, stdin)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// ParsedHeader is one -H value split on the first colon.
type ParsedHeader struct {
	Name, Value string
}

// parseHeaders splits each `Name: Value` entry on the first colon.
func parseHeaders(specs []string) ([]ParsedHeader, error) {
	out := make([]ParsedHeader, 0, len(specs))
	for _, spec := range specs {
		colon := strings.IndexByte(spec, ':')
		if colon < 0 {
			return nil, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				"header must be `Name: Value`: "+spec).WithField("header")
		}
		name := strings.TrimSpace(spec[:colon])
		val := strings.TrimSpace(spec[colon+1:])
		if name == "" {
			return nil, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				"header name must not be empty: "+spec).WithField("header")
		}
		out = append(out, ParsedHeader{Name: name, Value: val})
	}
	return out, nil
}

// methodDefaultsToPost reports whether the CLI should default --method to
// POST given a field count. Matches `gh api` / `vercel api` behavior.
func methodDefaultsToPost(fieldCount int, hasInput, hasBody bool) bool {
	return fieldCount > 0 || hasInput || hasBody
}
