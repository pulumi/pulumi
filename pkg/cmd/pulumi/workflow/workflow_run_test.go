// Copyright 2016, Pulumi Corporation.
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

package workflow

import "testing"

func TestParseInputJSON(t *testing.T) {
	t.Parallel()

	value, err := parseInputJSON(`{"message":"hello","repeat":3}`)
	if err != nil {
		t.Fatalf("parseInputJSON failed: %v", err)
	}
	if got := value["message"]; got != "hello" {
		t.Fatalf("unexpected message value: %#v", got)
	}
	if got := value["repeat"]; got != float64(3) {
		t.Fatalf("unexpected repeat value: %#v", got)
	}
}

func TestParseInputJSONInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseInputJSON(`not-json`)
	if err == nil {
		t.Fatalf("expected parseInputJSON to fail")
	}
}
