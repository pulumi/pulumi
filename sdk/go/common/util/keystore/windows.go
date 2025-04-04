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

//go:build windows
// +build windows

package keystore

import (
	"errors"
	"fmt"
	"github.com/danieljoos/wincred"
)

func storeKey(keyID string, key []byte) error {
	cred := wincred.NewGenericCredential(fmt.Sprintf("pulumi:keystore:%s", keyID))
	cred.CredentialBlob = key
	return cred.Write()
}

func getKey(keyID string) ([]byte, error) {
	cred, err := wincred.GetGenericCredential(fmt.Sprintf("pulumi:keystore:%s", keyID))
	if errors.Is(err, wincred.ErrElementNotFound) {
		// Not Found
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cred.CredentialBlob, nil
}
