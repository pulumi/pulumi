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

package schema

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLoader struct {
	GetPackageF func(context.Context, *PackageDescriptor) (PackageReference, error)
}

func (m *mockLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := m.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (m *mockLoader) LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error) {
	ref, err := m.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (m *mockLoader) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	return m.LoadPackageReferenceV2(context.TODO(), &PackageDescriptor{
		Name:    pkg,
		Version: version,
	})
}

func (m *mockLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *PackageDescriptor,
) (PackageReference, error) {
	return m.GetPackageF(ctx, descriptor)
}

func TestCachedLoader(t *testing.T) {
	t.Parallel()

	calls := 0
	mockLoader := &mockLoader{
		GetPackageF: func(context.Context, *PackageDescriptor) (PackageReference, error) {
			calls++
			return nil, nil
		},
	}

	loader := NewCachedLoader(mockLoader)

	_, err := loader.LoadPackageReference("pkg", nil)
	require.NoError(t, err)

	_, err = loader.LoadPackageReference("pkg", nil)
	require.NoError(t, err)

	assert.Equal(t, 1, calls)
}
