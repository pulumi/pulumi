// Copyright 2019-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package npm

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// denoJSONLinks reads the "links" array from the deno.json file in dir.
// Used in tests to verify the contents of the links field.
func denoJSONLinks(dir string) ([]string, error) {
	denoJSONPath := filepath.Join(dir, "deno.json")
	content, err := os.ReadFile(denoJSONPath)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(content), &config); err != nil {
		return nil, err
	}
	existing, ok := config["links"]
	if !ok {
		return nil, nil
	}
	arr, ok := existing.([]interface{})
	if !ok {
		return nil, nil
	}
	var links []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			links = append(links, s)
		}
	}
	return links, nil
}
