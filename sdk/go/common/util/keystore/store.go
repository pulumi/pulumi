// Copyright 2025, Pulumi Corporation.
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

package keystore

import (
	cryptorand "crypto/rand"
)

type KeyStore struct {
	Name string
}

// TODO: Implement a static (or locally calculated key) for non-pulumi-cli and non-esc-cli executables or if these
//       executables struggle to access keychain/keyring/credential manager.

func (ks KeyStore) GetOrCreateKey() ([]byte, error) {
	key, err := getKey(ks.Name)
	if err != nil {
		return nil, err
	}
	if key == nil {
		key = make([]byte, 32)
		if _, err = cryptorand.Read(key); err != nil {
			return nil, err
		}
		if err = storeKey(ks.Name, key); err != nil {
			return nil, err
		}
	}
	return key, nil
}
