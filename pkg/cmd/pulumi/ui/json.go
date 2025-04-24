// Copyright 2024, Pulumi Corporation.
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

package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MakeJSONString turns the given value into a JSON string. If multiline is true, the JSON will be formatted with
// indentation and a trailing newline.
func MakeJSONString(v interface{}, multiline bool) (string, error) {
	var out bytes.Buffer

	// json.Marshal escapes HTML characters, which we don't want,
	// so change that with json.NewEncoder.
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)

	if multiline {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(v); err != nil {
		return "", err
	}

	// json.NewEncoder always adds a trailing newline. Remove it.
	bs := out.Bytes()
	if !multiline {
		if n := len(bs); n > 0 && bs[n-1] == '\n' {
			bs = bs[:n-1]
		}
	}

	return string(bs), nil
}

// PrintJSON simply prints out some object, formatted as JSON, using standard indentation.
func PrintJSON(v interface{}) error {
	return FprintJSON(os.Stdout, v)
}

// FprintJSON simply prints out some object, formatted as JSON, using standard indentation.
func FprintJSON(w io.Writer, v interface{}) error {
	jsonStr, err := MakeJSONString(v, true /* multi line */)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, jsonStr)
	return err
}
