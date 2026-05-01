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

package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// parseField parses a single `key=value` spec into a ParsedField. typed=true
// runs the value through json.Unmarshal so JSON literals decode to their
// native Go types; anything that fails to parse, including bare unquoted strings,
// falls through as a plain string, which is the gh-style ergonomic for `-F note=hello`.
// `@path` (or `@-` for stdin) is honoured for both typed and raw fields.
// typed=false always emits the value verbatim as a string (for `-f`).
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
	var parsed any
	if err := json.Unmarshal([]byte(val), &parsed); err == nil {
		return ParsedField{Key: key, Value: parsed}, nil
	}
	return ParsedField{Key: key, Value: val}, nil
}

// readAtSource implements `@path` and `@-` file-or-stdin reading. Callers
// must pass a non-nil stdin Reader when `@-` is used
func readAtSource(source string, stdin io.Reader) ([]byte, error) {
	if source == "-" {
		if stdin == nil {
			return nil, NewAPIError(cmdutil.ExitInternalError, ErrToolError,
				"@- field requires a stdin reader; caller passed nil")
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
		rawName, rawVal, ok := strings.Cut(spec, ":")
		if !ok {
			return nil, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				"header must be `Name: Value`: "+spec).WithField("header")
		}
		name := strings.TrimSpace(rawName)
		val := strings.TrimSpace(rawVal)
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

// buildAPIHeaders folds contentType / accept / user `-H` headers into a
// single http.Header for httpstate Client.RawCall. User headers win over
// the encoder defaults (Accept, Content-Type); user-supplied Authorization
// is dropped — RawCall pins it from the Client's token.
func buildAPIHeaders(contentType, accept string, headers []ParsedHeader) http.Header {
	h := http.Header{}
	if accept != "" {
		h.Set("Accept", accept)
	} else {
		h.Set("Accept", "application/json")
	}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	for _, ph := range headers {
		if strings.EqualFold(ph.Name, "Authorization") {
			continue
		}
		h.Set(ph.Name, ph.Value)
	}
	return h
}
