// Copyright 2016-2025, Pulumi Corporation.
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
	"context"
	"encoding/json"

	"github.com/pulumi/esc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func ToConfigMap(
	ctx context.Context, environmentPulumiConfig esc.Value, defaultNamespace string, encrypter Encrypter,
) (Map, error) {
	plaintextMap := map[Key]plaintext{}
	if envMap, ok := environmentPulumiConfig.Value.(map[string]esc.Value); ok {
		for rawKey, value := range envMap {
			key, err := ParseConfigKey(defaultNamespace, rawKey)
			if err != nil {
				return nil, err
			}
			plaintextMap[key] = toPlaintext(value)
		}
	}

	objectMap, err := encryptMap(ctx, plaintextMap, encrypter)
	if err != nil {
		return nil, err
	}

	result := Map{}
	for k, obj := range objectMap {
		v, err := obj.marshalValue()
		if err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, nil
}

func toPlaintext(v esc.Value) plaintext {
	if v.Unknown {
		if v.Secret {
			return newSecurePlaintext("[unknown]")
		}
		return newPlaintext("[unknown]")
	}

	switch repr := v.Value.(type) {
	case nil:
		return plaintext{}
	case bool:
		return newPlaintext(repr)
	case json.Number:
		if i, err := repr.Int64(); err == nil {
			return newPlaintext(i)
		} else if f, err := repr.Float64(); err == nil {
			return newPlaintext(f)
		}
		// TODO(pdg): this disagrees with config unmarshaling semantics. Should probably fail.
		return newPlaintext(string(repr))
	case string:
		if v.Secret {
			return newSecurePlaintext(repr)
		}
		return newPlaintext(repr)
	case []esc.Value:
		vs := make([]plaintext, len(repr))
		for i, v := range repr {
			vs[i] = toPlaintext(v)
		}
		return newPlaintext(vs)
	case map[string]esc.Value:
		vs := make(map[string]plaintext, len(repr))
		for k, v := range repr {
			vs[k] = toPlaintext(v)
		}
		return newPlaintext(vs)
	default:
		contract.Failf("unexpected environments value of type %T", repr)
		return plaintext{}
	}
}
