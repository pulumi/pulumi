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

package testing

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func RandomStackName() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	contract.AssertNoErrorf(err, "failed to generate random stack name")
	return "test" + hex.EncodeToString(b)
}

// LogIfVerbose logs a message only if PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT is set to "true".
func LogIfVerbose(t *testing.T, name, message string) {
	if os.Getenv("PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT") != "true" {
		return
	}
	t.Logf("%s: %s", name, message)
}
