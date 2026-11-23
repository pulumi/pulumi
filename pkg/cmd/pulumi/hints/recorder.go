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

// RecordProviderHint offers the hint that a provider reported for a failure, such as dynamic credentials. Commands
// set it as the HintRecorder of the updates they run.
func RecordProviderHint(h plugin.Hint) { defaultManager.recordProviderHint(h) }

func (m *manager) recordProviderHint(h plugin.Hint) {
	if h.Kind != plugin.HintDynamicCredentials || h.Message == "" {
		return
	}
	m.offer(ESCCredentials(h.Message))
}
