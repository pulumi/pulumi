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
	"iter"
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// Read only methods on the registry.
type Registry interface {
	// Retrieve metadata about a specific package.
	//
	// {source}/{publisher}/{name} should form the identifier that describes the
	// desired package.
	//
	// If version is nil, it will default to latest.
	//
	// Implementations of GetPackage should return `apitype.PackageMetadata{}, err`
	// such that `errors.Is(err, ErrNotFound{})` returns true when the arguments to
	// GetPackage do not point to a package.
	GetPackage(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.PackageMetadata, error)
	// Retrieve a list of packages.
	//
	// If name is non-nil, it will filter to accessible packages that exactly match
	// */*/{name}.
	//
	// Implementations of SearchByName should return an empty iterator and nil if
	// there are no matching packages in the Registry.
	SearchByName(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]
}

var ErrNotFound = NotFoundError{}

type NotFoundError struct{}

func (err NotFoundError) Error() string {
	return "not found"
}

type registryKey struct{}

func Set(ctx context.Context, registry Registry) context.Context {
	return context.WithValue(ctx, registryKey{}, registry)
}

func Get(ctx context.Context) Registry {
	v := ctx.Value(registryKey{})
	if v == nil {
		return nil
	}
	return v.(Registry)
}

// NewOnDemandRegistry allows delaying the creation of a registry until it's necessary.
//
// If f returns an error, all calls to registry functions will return that error.
func NewOnDemandRegistry(f func() (Registry, error)) Registry {
	return &onDemandRegistry{sync.OnceValues(f)}
}

type onDemandRegistry struct{ factory func() (Registry, error) }

func (r *onDemandRegistry) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	impl, err := r.factory()
	if err != nil {
		return apitype.PackageMetadata{}, err
	}
	return impl.GetPackage(ctx, source, publisher, name, version)
}

func (r *onDemandRegistry) SearchByName(
	ctx context.Context, name *string,
) iter.Seq2[apitype.PackageMetadata, error] {
	impl, err := r.factory()
	if err != nil {
		return func(consumer func(apitype.PackageMetadata, error) bool) {
			consumer(apitype.PackageMetadata{}, err)
		}
	}
	return impl.SearchByName(ctx, name)
}
