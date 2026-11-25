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

package state

import (
	"bytes"
	"os"
	"strings"
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotFrontendRoundTrip(t *testing.T) {
	t.Parallel()
	encoder := &jsonSnapshotEncoder{}

	snapshot := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Magic:   "a985ef3cf45426591e732ba9a8c59847d8d3bfcf747713e1e6bb4589b89a75ed",
			Version: "3.74.1-dev.0",
		},
		Resources: []*pkgresource.State{
			{
				URN:  resource.URN("urn:pulumi:dev::random::pulumi:pulumi:Stack::random-dev"),
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.PropertyMap{
					resource.PropertyKey("name"): resource.NewProperty("fancy-pig"),
				},
			},
			{
				URN:    resource.URN("urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2"),
				Type:   "pulumi:providers:random",
				Custom: true,
				ID:     resource.ID("ed72fad1-9a82-49d7-b09f-1659b7a3c7db"),
				Inputs: resource.PropertyMap{
					resource.PropertyKey("version"): resource.NewProperty("4.13.2"),
				},
				Outputs: resource.PropertyMap{
					resource.PropertyKey("aversion"): resource.NewProperty("4.13.2"),
					resource.PropertyKey("version"):  resource.NewProperty("4.13.2"),
				},
			},
			{
				URN:    resource.URN("urn:pulumi:dev::random::random:index/randomPet:RandomPet::username-1"),
				Type:   "random:index/randomPet:RandomPet",
				Custom: true,
				ID:     resource.ID("wondrous-doe"),
				Outputs: resource.PropertyMap{
					resource.PropertyKey("id"):        resource.NewProperty("wondrous-doe"),
					resource.PropertyKey("length"):    resource.NewProperty(2.0),
					resource.PropertyKey("separator"): resource.NewProperty("-"),
				},
				Parent:   resource.URN("urn:pulumi:dev::random::pulumi:pulumi:Stack::random-dev"),
				Provider: "urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2::ed72fad1-9a82-49d7-b09f-1659b7a3c7db",
			},
		},
	}

	// Convert the snapshot to text.
	text, err := encoder.SnapshotToText(snapshot)
	require.NoError(t, err)
	assert.NotEmpty(t, text)

	// Convert the text back to a snapshot.
	ctx := t.Context()
	roundTrippedSnapshot, err := encoder.TextToSnapshot(ctx, text)
	require.NoError(t, err)

	// Convert the snapshot back to text.
	roundTrippedText, err := encoder.SnapshotToText(roundTrippedSnapshot)
	require.NoError(t, err)
	assert.NotEmpty(t, roundTrippedText)

	// The round-tripped snapshot text should be the same as the original.
	assert.Equal(t, text, roundTrippedText)
}

func TestOpenInEditorMultiPart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		command  string
		filename string
	}{
		{
			name:     "Simple command",
			command:  "echo",
			filename: "filename.txt",
		},
		{
			name:     "Command with arguments",
			command:  "echo Hello",
			filename: "filename.txt",
		},
		{
			name:     "Complex command",
			command:  "echo --foo --bar a",
			filename: "filename.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := openInEditorInternal(tt.command, tt.filename)
			require.NoError(t, err)
		})
	}
}

func TestValidateAndPrintStateShowsDiff(t *testing.T) {
	t.Parallel()

	snapshot := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Magic:   "a985ef3cf45426591e732ba9a8c59847d8d3bfcf747713e1e6bb4589b89a75ed",
			Version: "3.74.1-dev.0",
		},
		Resources: []*pkgresource.State{
			{
				URN:  resource.URN("urn:pulumi:dev::random::pulumi:pulumi:Stack::random-dev"),
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.PropertyMap{
					resource.PropertyKey("name"): resource.NewProperty("fancy-pig"),
				},
			},
		},
	}

	f, err := newSnapshotBuffer(".json", &jsonSnapshotEncoder{}, snapshot)
	require.NoError(t, err)
	defer f.Cleanup()

	edited := strings.Replace(string(f.originalText), "fancy-pig", "happy-cat", 1)
	require.NoError(t, os.WriteFile(f.Name(), []byte(edited), 0o600))

	var stdout bytes.Buffer
	cmd := &stateEditCmd{Stdout: &stdout, Colorizer: colors.Never}
	news, err := cmd.validateAndPrintState(t.Context(), f)
	require.NoError(t, err)
	require.Equal(t, resource.NewProperty("happy-cat"), news.Resources[0].Outputs["name"])

	require.Equal(t, `State changes:
--- original
+++ edited
@@ -10,7 +10,7 @@
       "custom": false,
       "type": "pulumi:pulumi:Stack",
       "outputs": {
-        "name": "fancy-pig"
+        "name": "happy-cat"
       }
     }
   ],

`, stdout.String())
}

func TestValidateAndPrintStateNoChange(t *testing.T) {
	t.Parallel()

	snapshot := &deploy.Snapshot{
		Manifest: deploy.Manifest{
			Magic:   "a985ef3cf45426591e732ba9a8c59847d8d3bfcf747713e1e6bb4589b89a75ed",
			Version: "3.74.1-dev.0",
		},
	}

	f, err := newSnapshotBuffer(".json", &jsonSnapshotEncoder{}, snapshot)
	require.NoError(t, err)
	defer f.Cleanup()

	var stdout bytes.Buffer
	cmd := &stateEditCmd{Stdout: &stdout, Colorizer: colors.Never}
	_, err = cmd.validateAndPrintState(t.Context(), f)
	require.ErrorIs(t, err, errNoStateChange)
	require.Empty(t, stdout.String())
}
