// Copyright 2016-2024, Pulumi Corporation.
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

package newcmd

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // changes directory
func TestErrorsOnNonHTTPBackend(t *testing.T) {
	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
	})

	testNewArgs := newArgs{
		aiPrompt:        "prompt",
		aiLanguage:      "typescript",
		interactive:     true,
		secretsProvider: "default",
	}

	assert.ErrorContains(t,
		runNew(
			context.Background(), testNewArgs,
		),
		"please log in to Pulumi Cloud to use Pulumi AI")
}
