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

//go:build darwin
// +build darwin

package keystore

import (
	"github.com/keybase/go-keychain"
	"sync"
)

// TODO: Cache as env variable? Required at all when app is signed?
var cacheMutex sync.Mutex
var cachedKey []byte

func storeKey(keyID string, key []byte) error {
	var err error
	item := keychain.NewItem()

	// This is recommended for items that need to be accessible only while the application is in the foreground.
	// Items with this attribute do not migrate to a new device.
	// Thus, after restoring from a backup of a different device, these items will not be present.
	var accessible keychain.Accessible
	accessible = keychain.AccessibleWhenUnlockedThisDeviceOnly

	// IF TouchID

	//options := keychain.AuthenticationContextOptions{AllowableReuseDuration: 10}
	//authContext := keychain.CreateAuthenticationContext(options)
	//err = item.SetAuthenticationContext(authContext)
	//if err != nil {
	//	return err
	//}
	//err = item.SetAccessControl(keychain.AccessControlFlagsUserPresence, accessible)
	//if err != nil {
	//	return err
	//}
	//err = item.SetUseDataProtectionKeychain(true)
	//if err != nil {
	//	return err
	//}

	// ELSE

	item.SetAccessible(accessible)

	// END

	// TODO: Evaluate notarization of the pulumi-cli and esc-cli so that both can access the same
	//       access group -> the same key -> the same stored creds.
	//item.SetAccessGroup("pulumi")

	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService("pulumi")
	item.SetAccount(keyID)
	item.SetLabel("pulumi:keystore")
	item.SetData(key)
	item.SetSynchronizable(keychain.SynchronizableNo)
	err = keychain.AddItem(item)
	if err == keychain.ErrorDuplicateItem {
		err = keychain.UpdateItem(item, item)
	}
	return err
}

// TODO: When changing code and re-building the keychain asks for permission to confidential information
// in keychain and again for access to key. Might be fixed when app is signed.
func getKey(keyID string) ([]byte, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if cachedKey != nil {
		return cachedKey, nil
	}

	query := keychain.NewItem()

	// IF TouchID

	//options := keychain.AuthenticationContextOptions{AllowableReuseDuration: 10}
	//authContext := keychain.CreateAuthenticationContext(options)
	//err := query.SetAuthenticationContext(authContext)
	//if err != nil {
	//	return nil, err
	//}
	//err = query.SetUseDataProtectionKeychain(true)
	//if err != nil {
	//	return nil, err
	//}

	// END

	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService("pulumi")
	query.SetAccount(keyID)
	query.SetLabel("pulumi:keystore")
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		cachedKey = nil
		return nil, err
	} else if len(results) != 1 {
		// Not Found
		cachedKey = nil
		return nil, nil
	} else {
		cachedKey = results[0].Data
		return cachedKey, nil
	}
}
