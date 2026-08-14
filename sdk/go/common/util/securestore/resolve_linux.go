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

//go:build linux

package securestore

// platformCandidates returns Linux backends in preference order: the Secret
// Service holding a TPM2-sealed blob where a TPM is present, the Secret
// Service with the raw key otherwise, and a TPM-sealed file when a TPM
// exists but no Secret Service is usable (headless servers).
func platformCandidates(allowPrompt bool, pulumiHome string) []backendImpl {
	ss := newKeyringStore(func() (Outcome, error) { return secretServicePrecheck(allowPrompt) })
	return []backendImpl{
		{id: BackendLinuxSecretServiceTPM, store: ss, wrap: tpmWrapper{}},
		{id: BackendLinuxSecretService, store: ss, wrap: rawWrapper{}},
		{id: BackendTPMFile, store: newTPMFileStore(pulumiHome), wrap: tpmWrapper{}},
	}
}
