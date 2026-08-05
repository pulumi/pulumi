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

//go:build darwin

package securestore

// platformCandidates returns macOS backends in preference order: the native
// SecItem backend (per-app ACL, Developer-ID-signed builds only) first, then
// the /usr/bin/security fallback that works for any binary. Both tiers apply
// the unlock prompt policy: silent cells never let securityd draw a dialog.
func platformCandidates(allowPrompt bool, _ string) []backendImpl {
	return []backendImpl{
		nativeKeychainBackend(allowPrompt),
		{
			id:    BackendMacOSSecurity,
			store: newKeyringStore(func() (Outcome, error) { return keychainPrecheck(allowPrompt) }),
			wrap:  rawWrapper{},
		},
	}
}
