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

package state

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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
		Resources: []*resource.State{
			{
				URN:  resource.URN("urn:pulumi:dev::random::pulumi:pulumi:Stack::random-dev"),
				Type: "pulumi:pulumi:Stack",
				Outputs: resource.PropertyMap{
					resource.PropertyKey("name"): resource.NewStringProperty("fancy-pig"),
				},
			},
			{
				URN:    resource.URN("urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2"),
				Type:   "pulumi:providers:random",
				Custom: true,
				ID:     resource.ID("ed72fad1-9a82-49d7-b09f-1659b7a3c7db"),
				Inputs: resource.PropertyMap{
					resource.PropertyKey("version"): resource.NewStringProperty("4.13.2"),
				},
				Outputs: resource.PropertyMap{
					resource.PropertyKey("aversion"): resource.NewStringProperty("4.13.2"),
					resource.PropertyKey("version"):  resource.NewStringProperty("4.13.2"),
				},
			},
			{
				URN:    resource.URN("urn:pulumi:dev::random::random:index/randomPet:RandomPet::username-1"),
				Type:   "random:index/randomPet:RandomPet",
				Custom: true,
				ID:     resource.ID("wondrous-doe"),
				Outputs: resource.PropertyMap{
					resource.PropertyKey("id"):        resource.NewStringProperty("wondrous-doe"),
					resource.PropertyKey("length"):    resource.NewNumberProperty(2),
					resource.PropertyKey("separator"): resource.NewStringProperty("-"),
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
	ctx := context.Background()
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := openInEditorInternal(tt.command, tt.filename)
			require.NoError(t, err)
		})
	}
}
