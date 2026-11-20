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

import "time"

// ESCCredentials suggests Pulumi ESC after a provider says that dynamic credentials may help with a failure. The
// provider writes the message, because it knows its cloud and how to set up ESC for it.
func ESCCredentials(message string) Hint {
	return Hint{
		ID:        "esc-credentials",
		Cooldown:  24 * time.Hour,
		Message:   message,
		CloudOnly: true,
		// A new user's first update failing on credentials is better served by the provider's own error.
		EstablishedStackOnly: true,
	}
}
