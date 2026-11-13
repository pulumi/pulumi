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

import "github.com/pulumi/pulumi/pkg/v3/resource/plugin"

// Recorder returns the recorder that turns the hints providers report for their failures, such as dynamic
// credentials, into this process's hints. Commands set it as the HintRecorder of the plugin contexts they create.
func Recorder() plugin.HintRecorder { return defaultManager }

// RecordHint implements plugin.HintRecorder. It offers the hint a provider sent, with the provider's message.
func (m *manager) RecordHint(hint plugin.Hint, message string) {
	if hint != plugin.HintDynamicCredentials || message == "" {
		return
	}
	m.offer(ESCCredentials(message))
}
