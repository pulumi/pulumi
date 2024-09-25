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

package backend

import (
	"context"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager interface {
	// Current returns the currently logged in backend instance for the given url.
	Current(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
	) (Backend, error)

	// Login starts the login process for the given URL. If there is already a logged-in backend, this is returned as-is.
	Login(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
		color colors.Colorization,
	) (Backend, error)
}
