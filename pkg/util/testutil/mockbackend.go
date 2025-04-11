// Copyright 2025, Pulumi Corporation.
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

package testutil

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
)

// MockBackendInstance sets the backend instance for the test and cleans it up after.
func MockBackendInstance(t *testing.T, b backend.Backend) {
	t.Cleanup(func() {
		cmdBackend.BackendInstance = nil
	})
	cmdBackend.BackendInstance = b
}

// MockLoginManager sets the login manager for the test and cleans it up after.
func MockLoginManager(t *testing.T, lm cmdBackend.LoginManager) {
	t.Cleanup(func() {
		cmdBackend.DefaultLoginManager = nil
	})
	cmdBackend.DefaultLoginManager = lm
}
