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

//go:build freebsd || linux || netbsd || openbsd || solaris || dragonfly
// +build freebsd linux netbsd openbsd solaris dragonfly

package keystore

import (
	"github.com/keybase/dbus"
	"github.com/keybase/go-keychain/secretservice"
	"strings"
	"sync"
)

// TODO: Cache as env variable?
var cacheMutex sync.Mutex
var cachedKey []byte

func storeKey(keyID string, key []byte) error {
	srv, err := secretservice.NewService()
	if err != nil {
		return err
	}
	session, err := srv.OpenSession(secretservice.AuthenticationDHAES)
	if err != nil {
		return err
	}
	defer srv.CloseSession(session)

	collection := secretservice.DefaultCollection

	secret, err := session.NewSecret(key)
	if err != nil {
		return err
	}

	err = srv.Unlock([]dbus.ObjectPath{collection})
	if err != nil {
		return err
	}

	// TODO: Check how credential sharing between pulumi-cli and esc-cli works on Linux
	_, err = srv.CreateItem(collection, secretservice.NewSecretProperties("pulumi:keystore", map[string]string{"account": keyID}), secret, secretservice.ReplaceBehaviorReplace)
	if err != nil {
		return err
	}

	return srv.LockItems([]dbus.ObjectPath{collection})
}

func getKey(keyID string) ([]byte, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if cachedKey != nil {
		return cachedKey, nil
	}

	srv, err := secretservice.NewService()
	if err != nil {
		return nil, err
	}
	session, err := srv.OpenSession(secretservice.AuthenticationDHAES)
	if err != nil {
		return nil, err
	}
	defer srv.CloseSession(session)

	collection := secretservice.DefaultCollection

	err = srv.Unlock([]dbus.ObjectPath{collection})
	if err != nil {
		return nil, err
	}

	items, err := srv.SearchCollection(collection, map[string]string{"account": keyID})
	if err != nil {
		if strings.Contains(err.Error(), "Object does not exist at path") {
			// Not Found
			cachedKey = nil
			return nil, nil
		}
		return nil, err
	}
	if len(items) == 0 {
		// Not Found
		cachedKey = nil
		return nil, nil
	}
	gotItem := items[0]
	key, err := srv.GetSecret(gotItem, *session)
	if err != nil {
		return nil, err
	}

	err = srv.LockItems([]dbus.ObjectPath{collection})
	if err != nil {
		return nil, err
	}

	cachedKey = key
	return key, nil
}
