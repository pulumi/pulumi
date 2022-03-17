// Copyright 2016-2021, Pulumi Corporation.
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

package auto

import (
	"fmt"
	"testing"
)

const testPermalink = "Permalink: https://gotest"

func TestGetPermalink(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		testee string
		want   string
		err    error
	}{
		"successful parsing": {testee: fmt.Sprintf("%s\n", testPermalink), want: "https://gotest"},
		"failed parsing":     {testee: testPermalink, err: ErrParsePermalinkFailed},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GetPermalink(test.testee)
			if err != nil {
				if test.err == nil || test.err != err {
					t.Errorf("got '%v', want '%v'", err, test.err)
				}
			}

			if got != test.want {
				t.Errorf("got '%s', want '%s'", got, test.want)
			}
		})
	}

}
