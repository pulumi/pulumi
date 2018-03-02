// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"
)

type Key struct {
	namespace string
	name      string
}

func MakeKey(namespace string, name string) Key {
	return Key{namespace: namespace, name: name}
}

func ParseKey(s string) (Key, error) {
	mm, err := tokens.ParseModuleMember(s)
	if err == nil {
		return fromModuleMember(mm)
	}
	if idx := strings.Index(s, ":"); idx > -1 {
		return Key{namespace: s[:idx], name: s[idx+1:]}, nil
	}

	return Key{}, errors.Errorf("could not parse %s as a configuration key", s)
}

func fromModuleMember(m tokens.ModuleMember) (Key, error) {
	if m.Module().Name() != tokens.ModuleName("config") {
		return Key{}, errors.Errorf("%s is not in config module", m)
	}

	return Key{
		namespace: m.Module().Package().String(),
		name:      m.Name().String(),
	}, nil
}

func (k Key) AsModuleMember() tokens.ModuleMember {
	return tokens.ModuleMember(k.namespace + ":config:" + k.name)
}

func (k Key) Namespace() string {
	return k.namespace
}

func (k Key) Name() string {
	return k.name
}

func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.AsModuleMember())
}

func (k *Key) UnmarshalJSON(b []byte) error {
	var mm tokens.ModuleMember
	if err := json.Unmarshal(b, &mm); err != nil {
		return errors.Wrap(err, "could not unmarshal key")
	}

	pk, err := fromModuleMember(mm)
	if err != nil {
		return err
	}

	k.namespace = pk.namespace
	k.name = pk.name
	return nil
}

func (k Key) MarshalYAML() (interface{}, error) {
	return k.AsModuleMember(), nil
}

func (k *Key) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var mm tokens.ModuleMember
	if err := unmarshal(&mm); err != nil {
		return errors.Wrap(err, "could not unmarshal key")
	}

	pk, err := fromModuleMember(mm)
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
