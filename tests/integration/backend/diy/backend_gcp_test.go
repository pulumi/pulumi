// Copyright 2024-2024, Pulumi Corporation.
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

package diy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // this test sets the global login state
func TestGcpLogin(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})

	if creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); creds == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS not set, skipping test")
	}

	cloudURL := "gs://pulumitesting"
	loginAndCreateStack(t, cloudURL)
}
