// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func (b *cloudBackend) OpenYAMLEnvironment(
	ctx context.Context,
	org string,
	yaml []byte,
	duration time.Duration,
) (*esc.Environment, []apitype.EnvironmentDiagnostic, error) {
	id, diags, err := b.client.OpenYAMLEnvironment(ctx, org, yaml, duration)
	if err != nil || len(diags) != 0 {
		return nil, diags, err
	}
	env, err := b.client.GetOpenEnvironment(ctx, org, "yaml", id)
	return env, nil, err
}
