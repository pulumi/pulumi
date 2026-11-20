// Copyright 2026, Pulumi Corporation.
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

package hints

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// Recorder returns a recorder that turns the hints providers report for their failures, such as dynamic
// credentials, into this process's hints for stack s. Commands set it as the HintRecorder of the plugin contexts
// they create.
func Recorder(s backend.Stack) plugin.HintRecorder { return &recorder{m: defaultManager, stack: s} }

type recorder struct {
	m     *manager
	stack backend.Stack
}

// RecordHint implements plugin.HintRecorder. It offers the hint a provider sent, with the provider's message.
func (r *recorder) RecordHint(hint plugin.Hint) {
	if hint.Kind != plugin.HintDynamicCredentials || hint.Message == "" {
		return
	}
	r.m.offerFor(ESCCredentials(hint.Message), r.stack)
}
