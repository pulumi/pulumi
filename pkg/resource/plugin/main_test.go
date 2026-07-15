// Copyright 2025, Pulumi Corporation.
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

package plugin

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		// These tests are skipped as part of enabling running unit tests on windows and MacOS in
		// https://github.com/pulumi/pulumi/pull/19653. These tests currently fail on Windows, and
		// re-enabling them is left as future work.
		// TODO[pulumi/pulumi#19675]: Re-enable tests on windows once they are fixed.
		fmt.Println("Skip tests on windows until they are fixed")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
