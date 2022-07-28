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

package testing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvOverrideGetCommandResults(t *testing.T) {
	t.Parallel()

	e := NewGoEnvironment(t)
	checkDebug := func(expect string) {
		stdout, stderr, err := e.GetCommandResults("bash", "-c", "echo $PULUMI_DEBUG_COMMANDS")
		stdout = strings.TrimSuffix(stdout, "\n")
		assert.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Equal(t, expect, stdout)

	}
	// We default PULUMI_DEBUG_COMMANDS to true
	checkDebug("true")
	// We can override the default
	e.SetEnvVars("PULUMI_DEBUG_COMMANDS=false")
	checkDebug("false")
}
