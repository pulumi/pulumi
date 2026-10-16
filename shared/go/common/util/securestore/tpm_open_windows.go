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

import (
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/windowstpm"
)

// openTPM opens the Windows TPM 2.0 via the TPM Base Services (tbs.dll)
// transport, which brokers access so we cannot collide with other TPM
// clients. windowstpm.Open verifies the device is a TPM 2.0.
func openTPM() (transport.TPMCloser, error) {
	tpm, err := windowstpm.Open()
	if err != nil {
		// windowstpm.Open can return a non-nil transport alongside an
		// error; treat any error as failure.
		if tpm != nil {
			tpm.Close()
		}
		return nil, err
	}
	return tpm, nil
}
