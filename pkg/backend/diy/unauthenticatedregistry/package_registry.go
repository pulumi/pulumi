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

package unauthenticatedregistry

import (
	"context"
	"errors"
	"iter"
	"net/http"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
)

type registryClient struct{ c *client.Client }

func New(sink diag.Sink, store env.Env) registry.Registry {
	url := "https://api.pulumi.com"
	if override := store.GetString(env.APIURL); override != "" {
		url = override
	}
	return registryClient{client.NewClient(url, "", false /* insecure */, sink)}
}

func (r registryClient) ListPackages(
	ctx context.Context, name *string,
) iter.Seq2[apitype.PackageMetadata, error] {
	return r.c.ListPackages(ctx, name)
}

func (r registryClient) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	meta, err := r.c.GetPackage(ctx, source, publisher, name, version)
	if apiErr := (&apitype.ErrorResponse{}); errors.As(err, &apiErr) && apiErr.Code == 404 {
		return meta, backenderr.NotFoundError{Err: err}
	}
	return meta, err
}

func (r registryClient) ListTemplates(
	ctx context.Context, name *string,
) iter.Seq2[apitype.TemplateMetadata, error] {
	return r.c.ListTemplates(ctx, name)
}

func (r registryClient) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	meta, err := r.c.GetTemplate(ctx, source, publisher, name, version)
	if apiErr := (&apitype.ErrorResponse{}); errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		return meta, backenderr.NotFoundError{Err: err}
	}
	return meta, err
}
