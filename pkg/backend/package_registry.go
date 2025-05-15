// Copyright 2025, Pulumi Corporation.
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

package backend

import (
	"context"
	"iter"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type PackageRegistry interface {
	// Publish publishes a package to the package registry.
	Publish(ctx context.Context, op apitype.PackagePublishOp) error

	// Retrieve metadata about a specific package.
	//
	// {source}/{publisher}/{name} should form the identifier that describes the
	// desired package.
	//
	// If version is nil, it will default to latest.
	GetPackage(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.PackageMetadata, error)

	// Retrieve a list of packages.
	//
	// If name is non-nil, it will filter to accessible packages that exactly match
	// */*/{name}.
	SearchByName(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]
}
