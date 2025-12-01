// Copyright 2020-2024, Pulumi Corporation.
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

package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbbreviateFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/Users/username/test-policy",
			expected: "/Users/username/test-policy",
		},
		{
			path:     "./..//test-policy",
			expected: "../test-policy",
		},
		{
			path: `/Users/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "/Users/.../twelve/test-policy",
		},
		{
			path: `nonrootdir/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "nonrootdir/username/.../twelve/test-policy",
		},
		{
			path: `C:/Documents and Settings/username/My Documents/averylongpath/` +
				`one/two/three/four/five/six/seven/eight/test-policy`,
			expected: "C:/Documents and Settings/.../eight/test-policy",
		},
		{
			path: `C:\Documents and Settings\username\My Documents\averylongpath\` +
				`one\two\three\four\five\six\seven\eight\test-policy`,
			expected: `C:\Documents and Settings\...\eight\test-policy`,
		},
	}

	for _, tt := range tests {
		actual := abbreviateFilePath(tt.path)
		assert.Equal(t, filepath.ToSlash(tt.expected), filepath.ToSlash(actual))
	}
}

func TestDeletingComponentResourceProducesResourceOutputsEvent(t *testing.T) {
	t.Parallel()

	cancelCtx, _ := cancel.NewContext(context.Background())

	acts := newUpdateActions(&Context{
		Cancel: cancelCtx,
	}, UpdateInfo{}, &deploymentOptions{})
	eventsChan := make(chan Event, 10)
	acts.Opts.Events.ch = eventsChan

	step := deploy.NewDeleteStep(&deploy.Deployment{}, map[resource.URN]bool{}, &resource.State{
		URN:      resource.URN("urn:pulumi:stack::project::my:example:Foo::foo"),
		ID:       "foo",
		Custom:   false,
		Provider: "unimportant",
	}, nil)
	acts.Seen[resource.URN("urn:pulumi:stack::project::my:example:Foo::foo")] = step

	err := acts.OnResourceStepPost(
		&mockSnapshotMutation{}, step, resource.StatusOK,
		nil, /* err */
	)
	require.NoError(t, err)

	//nolint:exhaustive // the default case is for test failures
	switch e := <-eventsChan; e.Type {
	case ResourceOutputsEvent:
		e, ok := e.Payload().(ResourceOutputsEventPayload)
		assert.True(t, ok)
		assert.True(t, e.Metadata.URN == "urn:pulumi:stack::project::my:example:Foo::foo")
	default:
		assert.Fail(t, "unexpected event type")
	}
}

type mockSnapshotMutation struct{}

func (msm *mockSnapshotMutation) End(step deploy.Step, successful bool) error { return nil }
