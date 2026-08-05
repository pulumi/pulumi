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

// *securestore.Store satisfies this implicitly.
type keyStore interface {
	Backend() securestore.Backend
	GetKey() ([]byte, error)
	GetOrCreateKey() ([]byte, error)
	DeleteKey() error
	FallbackReason() error
}

type keyStores interface {
	Resolve(mode securestore.Mode) (keyStore, error)
	ForBackend(id securestore.Backend) (keyStore, error)
}

// Wiring point, so no test double need be exported from securestore.
var stores keyStores = secureStores{}

type secureStores struct{}

// Deliberately GetPulumiHomeDir, not GetPulumiPath: the key file's location
// must be stable, never redirected to a per-session agent directory.
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
