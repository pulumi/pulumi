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

//go:build windows

package securestore

// platformCandidates returns Windows backends in preference order: the
// Credential Manager holding a TPM-wrapped blob where a TPM is present, the
// Credential Manager with the raw key otherwise, and a TPM-sealed file when
// a TPM exists but the credential store is unusable (e.g. SSH sessions).
func platformCandidates(bool) []backendImpl {
	return []backendImpl{
		{id: BackendWindowsCredManTPM, store: newKeyringStore(nil), wrap: tpmWrapper{}},
		{id: BackendWindowsCredMan, store: newKeyringStore(nil), wrap: rawWrapper{}},
		{id: BackendTPMFile, store: newTPMFileStore(), wrap: tpmWrapper{}},
	}
}
