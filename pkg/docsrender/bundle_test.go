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

package docsrender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIDocsPaths(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/registry/packages/aws/api-docs", APIDocsBasePath("aws"))
	assert.Equal(t, "/registry/packages/aws/api-docs/s3", APIDocsModulePath("aws", "", "s3"))
	assert.Equal(t,
		"/registry/packages/aws/api-docs/ec2/transitgateway",
		APIDocsModulePath("aws", "ec2", "transitgateway"))
	assert.Equal(t, "/registry/packages/aws/api-docs/s3/bucket", APIDocsEntryPath("aws", "s3/bucket"))
	assert.Equal(t, "/registry/packages/aws/api-docs/rootresource", APIDocsEntryPath("aws", "rootresource"))
}

func TestAbsolutizeBundleLinks(t *testing.T) {
	t.Parallel()

	t.Run("rewrites relative links", func(t *testing.T) {
		t.Parallel()
		md := "- [Bucket](bucket/)\n- [Instance](ec2/instance/)\n"
		got := absolutizeBundleLinks(md, "aws")
		assert.Contains(t, got, "[Bucket](/registry/packages/aws/api-docs/bucket/)")
		assert.Contains(t, got, "[Instance](/registry/packages/aws/api-docs/ec2/instance/)")
	})

	t.Run("leaves absolute paths alone", func(t *testing.T) {
		t.Parallel()
		md := "[Other](/registry/packages/other)"
		got := absolutizeBundleLinks(md, "aws")
		assert.Contains(t, got, "[Other](/registry/packages/other)")
	})

	t.Run("leaves external URLs alone", func(t *testing.T) {
		t.Parallel()
		md := "[Site](https://example.com/path)"
		got := absolutizeBundleLinks(md, "aws")
		assert.Contains(t, got, "[Site](https://example.com/path)")
	})

	t.Run("leaves anchors alone", func(t *testing.T) {
		t.Parallel()
		md := "[Section](#heading)"
		got := absolutizeBundleLinks(md, "aws")
		assert.Contains(t, got, "[Section](#heading)")
	})

	t.Run("returns input unchanged when nothing to rewrite", func(t *testing.T) {
		t.Parallel()
		md := "Just text, [no](https://x.io) relative links."
		got := absolutizeBundleLinks(md, "aws")
		assert.Equal(t, md, got)
	})

	t.Run("handles empty inputs", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", absolutizeBundleLinks("", "aws"))
		assert.Equal(t, "x", absolutizeBundleLinks("x", ""))
	})

	t.Run("output is recognized by NumberLinks", func(t *testing.T) {
		t.Parallel()
		md := "- [Bucket](bucket/)\n- [Policy](bucketpolicy/)\n"
		rewritten := absolutizeBundleLinks(md, "aws")
		_, links := NumberLinks(rewritten)
		require.Len(t, links, 2)
	})
}
