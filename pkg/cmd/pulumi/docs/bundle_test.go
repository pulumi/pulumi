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

package docs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleNavItems(t *testing.T) {
	t.Parallel()
	bundle := &docsrender.CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket":                "# Bucket\n\nContent",
			"s3/bucketPolicy":          "# BucketPolicy\n\nContent",
			"ec2/instance":             "# Instance\n\nContent",
			"ec2/vpc":                  "# Vpc\n\nContent",
			"ec2/transitgateway/route": "# Route\n\nContent",
			"rootresource":             "# RootResource\n\nContent",
		},
		Functions: map[string]string{
			"s3/getBucket": "# getBucket\n\nContent",
			"rootfunc":     "# rootFunc\n\nContent",
		},
	}

	t.Run("module level - s3", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "s3", "aws")

		// Should have: Bucket, BucketPolicy (resources), getBucket (function)
		require.Len(t, items, 3)

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}
		assert.Contains(t, labels, "🔗 Bucket")
		assert.Contains(t, labels, "🔗 BucketPolicy")
		assert.Contains(t, labels, "🔗 getBucket")

		// Check paths
		for _, item := range items {
			assert.True(t, len(item.path) > 0)
			assert.Contains(t, item.path, "registry/packages/aws/api-docs/s3/")
		}
	})

	t.Run("module level - ec2 with sub-modules", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "ec2", "aws")

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}

		// Should have: transitgateway (sub-module drill), Instance, Vpc (resources)
		assert.Contains(t, labels, "🔗 transitgateway"+navDrill)
		assert.Contains(t, labels, "🔗 Instance")
		assert.Contains(t, labels, "🔗 Vpc")
	})

	t.Run("root level - empty prefix", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "", "aws")

		labels := make([]string, len(items))
		for i, item := range items {
			labels[i] = item.label
		}

		// Should have sub-modules: ec2, s3 (drill), plus root-level: RootResource, rootFunc
		assert.Contains(t, labels, "🔗 ec2"+navDrill)
		assert.Contains(t, labels, "🔗 s3"+navDrill)
		assert.Contains(t, labels, "🔗 RootResource")
		assert.Contains(t, labels, "🔗 rootFunc")
	})

	t.Run("nested module", func(t *testing.T) {
		t.Parallel()
		items := BundleNavItems(bundle, "ec2/transitgateway", "aws")

		require.Len(t, items, 1)
		assert.Equal(t, "🔗 Route", items[0].label)
		assert.Equal(t, "registry/packages/aws/api-docs/ec2/transitgateway/route", items[0].path)
	})

	t.Run("empty bundle", func(t *testing.T) {
		t.Parallel()
		empty := &docsrender.CLIDocsBundle{}
		items := BundleNavItems(empty, "s3", "aws")
		assert.Empty(t, items)
	})
}

func TestBundleSectionNav(t *testing.T) {
	t.Parallel()

	bundle := &docsrender.CLIDocsBundle{
		Package: "aws",
		Resources: map[string]string{
			"s3/bucket":    "# Bucket\n\nContent",
			"rootresource": "# RootResource\n\nContent",
		},
		Functions: map[string]string{
			"rootfunc": "# rootFunc\n\nContent",
		},
	}

	t.Run("returns nav for sections", func(t *testing.T) {
		t.Parallel()
		nav := BundleSectionNav(bundle, "", "aws")
		assert.Contains(t, nav, docsrender.SectionModules)
		assert.Contains(t, nav, docsrender.SectionResources)
		assert.Contains(t, nav, docsrender.SectionFunctions)
		require.Len(t, nav[docsrender.SectionModules], 1)
		assert.Contains(t, nav[docsrender.SectionModules][0].label, "s3")
	})

	t.Run("empty bundle returns empty map", func(t *testing.T) {
		t.Parallel()
		empty := &docsrender.CLIDocsBundle{}
		nav := BundleSectionNav(empty, "", "test")
		assert.Empty(t, nav)
	})
}
