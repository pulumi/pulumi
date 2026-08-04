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

package workspace

import "github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"

// keyStore is the slice of the secure store this package calls: the key that
// encrypts the credentials file, and which mechanism protects it.
// *securestore.Store satisfies it implicitly.
type keyStore interface {
	Backend() securestore.Backend
	GetKey() ([]byte, error)
	GetOrCreateKey() ([]byte, error)
	DeleteKey() error
	FallbackReason() error
}

// keyStores resolves the store to use, either by mode for writes or by the
// backend an existing envelope records.
type keyStores interface {
	Resolve(mode securestore.Mode) (keyStore, error)
	ForBackend(id securestore.Backend) (keyStore, error)
}

// stores is the wiring point for this package. Production uses the real
// secure store; tests substitute a fake, so no test double is exported from
// securestore itself.
var stores keyStores = secureStores{}

type secureStores struct{}

// pulumiHome resolves the directory securestore keeps file-based key material
// in. Deliberately GetPulumiHomeDir and not GetPulumiPath: the key file's
// location must be stable, never redirected to a per-session agent directory.
// An empty result just makes the file-based backend report itself unusable.
func pulumiHome() string {
	dir, err := GetPulumiHomeDir()
	if err != nil {
		return ""
	}
	return dir
}

func (secureStores) Resolve(mode securestore.Mode) (keyStore, error) {
	st, err := securestore.Resolve(mode, pulumiHome())
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (secureStores) ForBackend(id securestore.Backend) (keyStore, error) {
	st, err := securestore.ForBackend(id, pulumiHome())
	if err != nil {
		return nil, err
	}
	return st, nil
}
