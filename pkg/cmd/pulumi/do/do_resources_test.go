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

package do

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestReferencedIdentsInPCL(t *testing.T) {
	t.Parallel()
	src := []byte(`
name = myBucket.arn
tags = { owner = teamRef.name }
options { provider = provider }
`)
	got := referencedIdentsInPCL(src, "test.pp")
	// The keys we care about are the roots of any traversal — attributes further down the path
	// (like `.arn`) shouldn't leak in.
	for _, want := range []string{"myBucket", "teamRef", "provider"} {
		_, ok := got[want]
		assert.True(t, ok, "expected %q in identifier set, got %v", want, got)
	}
	_, hasArn := got["arn"]
	assert.False(t, hasArn, "attribute names must not be treated as roots")
}

func TestFilterReferencesByUsage(t *testing.T) {
	t.Parallel()
	refs := map[string]string{"a": "urn:a", "b": "urn:b", "c": "urn:c"}
	used := map[string]struct{}{"a": {}, "c": {}, "other": {}}
	assert.Equal(t, map[string]string{"a": "urn:a", "c": "urn:c"},
		filterReferencesByUsage(refs, used))
	// A nil `used` (e.g. from an unparseable file) leaves refs unchanged so we don't accidentally
	// strip everything on a parse failure.
	assert.Equal(t, refs, filterReferencesByUsage(refs, nil))
}

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdShowResourcesSubcommand(t *testing.T) {
	bucket := resource.URN("urn:pulumi:dev::proj::aws:s3/bucket:Bucket::myBucket")
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{Type: bucket.Type(), URN: bucket, Custom: true},
		},
	})

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, nil, testHost, panicLoadConverterPlugin, nil)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"show-resources"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "myBucket")
	assert.Contains(t, stdout.String(), string(bucket))
}
