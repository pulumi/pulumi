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

package httpstate

import (
	ctx "context"
	"errors"
	"iter"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type cloudPackageRegistry struct {
	cl *client.Client
}

func newCloudPackageRegistry(cl *client.Client) *cloudPackageRegistry {
	return &cloudPackageRegistry{
		cl: cl,
	}
}

var _ backend.PackageRegistry = (*cloudPackageRegistry)(nil)

func (r *cloudPackageRegistry) Publish(ctx ctx.Context, op apitype.PackagePublishOp) error {
	return r.cl.PublishPackage(ctx, op)
}

func (r *cloudPackageRegistry) SearchByName(
	ctx ctx.Context, name *string,
) iter.Seq2[apitype.PackageMetadata, error] {
	return r.cl.SearchByName(ctx, name)
}

func (r *cloudPackageRegistry) GetPackage(
	ctx ctx.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	meta, err := r.cl.GetPackage(ctx, source, publisher, name, version)
	if apiErr := (&apitype.ErrorResponse{}); errors.As(err, &apiErr) && apiErr.Code == 404 {
		return meta, backenderr.NotFoundError{Err: err}
	}
	return meta, err
}
