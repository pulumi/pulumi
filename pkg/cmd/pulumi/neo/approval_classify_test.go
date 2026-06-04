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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAffirmative(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"y", "yes", "YES", "ok", "okay", "approve", "approved", "confirm",
		"confirmed", "proceed", "go ahead", "  Go   Ahead ", "go for it",
		"do it", "ship it", "lgtm", "go on", "all right", "alright",
	} {
		assert.True(t, isAffirmative(s), "%q should be affirmative", s)
	}
	for _, s := range []string{"", "no", "cancel", "not on prod", "yes but only on dev"} {
		assert.False(t, isAffirmative(s), "%q should not be affirmative", s)
	}
}
