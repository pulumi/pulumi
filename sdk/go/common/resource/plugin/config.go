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

package plugin

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func toConfig(prop property.Value) interface{} {
	if prop.IsNull() {
		return nil
	}
	if prop.IsString() {
		return prop.AsString()
	}
	if prop.IsBool() {
		return prop.AsBool()
	}
	if prop.IsNumber() {
		return prop.AsNumber()
	}
	if prop.IsArray() {
		list := []interface{}{}
		prop.AsArray().All(func(i int, v property.Value) bool {
			list = append(list, toConfig(v))
			return true
		})
		return list
	}
	if prop.IsMap() {
		obj := map[string]interface{}{}
		prop.AsMap().All(func(s string, v property.Value) bool {
			obj[s] = toConfig(v)
			return true
		})
		return obj
	}
	contract.Failf("unexpected property type %+v", prop)
	return nil
}

// PropertyMapToConfig converts a map of property values to a map of strings that can be passed via envvar strings to a
// plugin.
func PropertyMapToConfig(props property.Map) (map[string]string, []string) {
	config := map[string]string{}
	secretKeys := []string{}
	props.All(func(s string, v property.Value) bool {
		if v.Secret() {
			secretKeys = append(secretKeys, s)
		}

		// At the top level if it's a string return it as is
		if v.IsString() {
			config[s] = v.AsString()
			return true
		}
		// Otherwise, convert it to a JSON style string
		value := toConfig(v)
		bytes, err := json.Marshal(value)
		contract.AssertNoErrorf(err, "failed to marshal config value %v", value)
		config[s] = string(bytes)
		return true
	})
	return config, secretKeys
}
