// Copyright 2017-2018, Pulumi Corporation.
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

package workspace

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Settings defines workspace settings shared amongst many related projects.
// nolint: lll
type Settings struct {
	Stack            tokens.QName                `json:"stack,omitempty" yaml:"env,omitempty"`     // an optional default stack to use.
	ConfigDeprecated map[tokens.QName]config.Map `json:"config,omitempty" yaml:"config,omitempty"` // optional workspace local configuration (overrides values in a project)
}
