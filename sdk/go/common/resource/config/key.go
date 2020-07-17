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
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type Key struct {
	namespace string
	name      string
}

// MustMakeKey constructs a config.Key for a given namespace and name. The namespace may not contain a `:`
func MustMakeKey(namespace string, name string) Key {
	contract.Requiref(!strings.Contains(":", namespace), "namespace", "may not contain a colon")
	return Key{namespace: namespace, name: name}
}

func ParseKey(s string) (Key, error) {
	// Keys can take on of two forms:
	//
	// - <namespace>:<name> (the preferred form)
	// - <namespace>:config:<name> (compat with an old requirement that every config value be in the "config" module)
	//
	// Where <namespace> and <name> may be any string of characters, excluding ':'.

	switch strings.Count(s, ":") {
	case 1:
		idx := strings.Index(s, ":")
		return Key{namespace: s[:idx], name: s[idx+1:]}, nil
	case 2:
		if mm, err := tokens.ParseModuleMember(s); err == nil {
			if mm.Module().Name() == tokens.ModuleName("config") {
				return Key{
					namespace: mm.Module().Package().String(),
					name:      mm.Name().String(),
				}, nil
			}
		}
	}

	return Key{}, errors.Errorf("could not parse %s as a configuration key "+
		"(configuration keys should be of the form `<namespace>:<name>`)", s)
}

func (k Key) Namespace() string {
	return k.namespace
}

func (k Key) Name() string {
	return k.name
}

func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

func (k *Key) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.Wrap(err, "could not unmarshal key")
	}

	pk, err := ParseKey(s)
	if err != nil {
		return err
	}

	k.namespace = pk.namespace
	k.name = pk.name
	return nil
}

func (k Key) MarshalYAML() (interface{}, error) {
	return k.String(), nil
}

func (k *Key) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return errors.Wrap(err, "could not unmarshal key")
	}

	pk, err := ParseKey(s)
	if err != nil {
		return err
	}

	k.namespace = pk.namespace
	k.name = pk.name
	return nil
}

func (k Key) String() string {
	return k.namespace + ":" + k.name
}

type KeyArray []Key

func (k KeyArray) Len() int {
	return len(k)
}

func (k KeyArray) Less(i int, j int) bool {
	if k[i].namespace != k[j].namespace {
		return strings.Compare(k[i].namespace, k[j].namespace) == -1
	}

	return strings.Compare(k[i].name, k[j].name) == -1
}

func (k KeyArray) Swap(i int, j int) {
	k[i], k[j] = k[j], k[i]
}
