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

package registry

import (
	"context"
	"io"
	"iter"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var _ Registry = Mock{}

type Mock struct {
	GetPackageF func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.PackageMetadata, error)

	ListPackagesF func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]

	GetTemplateF func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.TemplateMetadata, error)

	ListTemplatesF func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error]

	DownloadTemplateF func(ctx context.Context, downloadURL string) (io.ReadCloser, error)
}

func (m Mock) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	if m.GetPackageF == nil {
		panic("registry.Mock.GetPackageF not implemented")
	}
	return m.GetPackageF(ctx, source, publisher, name, version)
}

func (m Mock) ListPackages(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
	if m.ListPackagesF == nil {
		panic("registry.Mock.ListPackagesF not implemented")
	}
	return m.ListPackagesF(ctx, name)
}

func (m Mock) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	if m.GetTemplateF == nil {
		panic("registry.Mock.GetTemplateF not implemented")
	}
	return m.GetTemplateF(ctx, source, publisher, name, version)
}

func (m Mock) ListTemplates(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
	if m.ListTemplatesF == nil {
		panic("registry.Mock.ListTemplatesF not implemented")
	}
	return m.ListTemplatesF(ctx, name)
}

func (m Mock) DownloadTemplate(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
	if m.ListTemplatesF == nil {
		panic("registry.Mock.DownloadTemplateF not implemented")
	}
	return m.DownloadTemplateF(ctx, downloadURL)
}
