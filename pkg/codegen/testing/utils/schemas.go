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

//nolint:revive // Legacy package name we don't want to change
package utils

import (
	"embed"
	"fmt"
	"io/fs"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
)

// schemaFiles holds the canonical set of provider schemas used by the codegen
// tests. Embedding them here lets downstream consumers (such as pulumi-yaml)
// resolve these schemas through the Go module rather than vendoring the
// pulumi/pulumi repository as a submodule.
//
//go:embed schemas
var schemaFiles embed.FS

// SchemaFS returns the embedded provider schemas, rooted so that each schema is
// named "<name>-<version>.json" (or ".yaml").
func SchemaFS() fs.FS {
	sub, err := fs.Sub(schemaFiles, "schemas")
	contract.AssertNoErrorf(err, "failed to open embedded schemas filesystem")
	return sub
}

// ReadSchema returns the embedded schema for the given package name and version,
// i.e. the contents of "<name>-<version>.json".
func ReadSchema(t *testing.T, name, version string) []byte {
	data, err := fs.ReadFile(SchemaFS(), fmt.Sprintf("%s-%s.json", name, version))
	require.NoError(t, err)
	return data
}
