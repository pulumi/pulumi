// Copyright 2016-2025, Pulumi Corporation.
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

package registry

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type mockRegistry struct {
	getPackage func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.PackageMetadata, error)
	listPackages func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]

	getTemplate func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.TemplateMetadata, error)
	listTemplates func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error]
}

func (r mockRegistry) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	return r.getPackage(ctx, source, publisher, name, version)
}

func (r mockRegistry) ListPackages(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
	return r.listPackages(ctx, name)
}

func (r mockRegistry) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	return r.getTemplate(ctx, source, publisher, name, version)
}

func (r mockRegistry) ListTemplates(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
	return r.listTemplates(ctx, name)
}

func TestOnDemandRegistry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		r := NewOnDemandRegistry(func() (Registry, error) {
			return mockRegistry{
				getPackage: func(
					ctx context.Context, source, publisher, name string, version *semver.Version,
				) (apitype.PackageMetadata, error) {
					assert.Equal(t, source, "src")
					assert.Equal(t, publisher, "pub")
					assert.Equal(t, name, "nm")
					assert.Equal(t, version, &semver.Version{Major: 3})
					return apitype.PackageMetadata{
						Name: "it worked",
					}, nil
				},
			}, nil
		})

		meta, err := r.GetPackage(ctx, "src", "pub", "nm", &semver.Version{Major: 3})
		require.NoError(t, err)
		assert.Equal(t, apitype.PackageMetadata{
			Name: "it worked",
		}, meta)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		markerErr := errors.New("marker error")

		r := NewOnDemandRegistry(func() (Registry, error) {
			return nil, markerErr
		})

		_, err := r.GetPackage(ctx, "src", "pub", "nm", &semver.Version{Major: 3})
		assert.ErrorIs(t, err, markerErr)
	})

	t.Run("once", func(t *testing.T) {
		t.Parallel()

		var count int
		r := NewOnDemandRegistry(func() (Registry, error) {
			count++
			return mockRegistry{
				getPackage: func(
					ctx context.Context, source, publisher, name string, version *semver.Version,
				) (apitype.PackageMetadata, error) {
					return apitype.PackageMetadata{
						Name:      name,
						Publisher: publisher,
						Source:    source,
					}, nil
				},
			}, nil
		})

		call := func() {
			result, err := r.GetPackage(ctx, "src", "pub", "nm", nil)
			if assert.NoError(t, err) {
				assert.Equal(t, "src", result.Source)
				assert.Equal(t, "pub", result.Publisher)
				assert.Equal(t, "nm", result.Name)
			}
		}

		call()
		call()
		call()

		assert.Equal(t, 1, count)
	})
}
