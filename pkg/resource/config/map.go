// Copyright 2016-2018, Pulumi Corporation.
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

package config

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Map is a bag of config stored in the settings file.
type Map map[Key]Value

// Decrypt returns the configuration as a map from module member to decrypted value.
func (m Map) Decrypt(decrypter Decrypter) (map[Key]string, error) {
	r := map[Key]string{}
	for k, c := range m {
		v, err := c.Value(decrypter)
		if err != nil {
			return nil, err
		}
		r[k] = v
	}
	return r, nil
}

// HasSecureValue returns true if the config map contains a secure (encrypted) value.
func (m Map) HasSecureValue() bool {
	for _, v := range m {
		if v.Secure() {
			return true
		}
	}

	return false
}

func (m Map) MarshalJSON() ([]byte, error) {
	rawMap := make(map[string]Value, len(m))
	for k, v := range m {
		rawMap[k.String()] = v
	}

	return json.Marshal(rawMap)
}

func (m *Map) UnmarshalJSON(b []byte) error {
	rawMap := make(map[string]Value)
	if err := json.Unmarshal(b, &rawMap); err != nil {
		return errors.Wrap(err, "could not unmarshal map")
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return errors.Wrap(err, "could not unmarshal map")
		}
		newMap[pk] = v
	}

	*m = newMap
	return nil
}

func (m Map) MarshalYAML() (interface{}, error) {
	rawMap := make(map[string]Value, len(m))
	for k, v := range m {
		rawMap[k.String()] = v
	}

	return rawMap, nil
}

func (m *Map) UnmarshalYAML(unmarshal func(interface{}) error) error {
	rawMap := make(map[string]Value)
	if err := unmarshal(&rawMap); err != nil {
		return errors.Wrap(err, "could not unmarshal map")
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return errors.Wrap(err, "could not unmarshal map")
		}
		newMap[pk] = v
	}

	*m = newMap
	return nil
}
