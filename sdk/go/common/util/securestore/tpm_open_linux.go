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

import (
	"errors"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

// openTPM opens the Linux TPM 2.0 character device. /dev/tpmrm0 (the kernel's
// TPM resource manager) is preferred: it virtualizes transient object slots
// so we cannot collide with other TPM clients. /dev/tpm0 is the raw device
// fallback for old kernels without the resource manager.
func openTPM() (transport.TPMCloser, error) {
	tpm, rmErr := linuxtpm.Open("/dev/tpmrm0")
	if rmErr == nil {
		return tpm, nil
	}
	tpm, rawErr := linuxtpm.Open("/dev/tpm0")
	if rawErr == nil {
		return tpm, nil
	}
	// Both errors name their device path, so report both.
	return nil, errors.Join(rmErr, rawErr)
}
