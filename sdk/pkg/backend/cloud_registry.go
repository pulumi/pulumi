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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
)

type CloudRegistry interface {
	registry.Registry
	// PublishPackage publishes a package to the registry.
	PublishPackage(ctx context.Context, op apitype.PackagePublishOp) error
	// PublishTemplate publishes a template to the registry.
	PublishTemplate(ctx context.Context, op apitype.TemplatePublishOp) error
}
