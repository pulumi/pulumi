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
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func mapPaths(t *testing.T, c Map) []Key {
	var paths []Key
	for k, v := range c {
		obj, err := v.ToObject()
		require.NoError(t, err)

		paths = append(paths, k)
		for _, p := range valuePaths(obj) {
			p = append(resource.PropertyPath{k.Name()}, p...)
			paths = append(paths, MustMakeKey(k.Namespace(), p.String()))
		}
	}
	return paths
}

func valuePaths(o any) []resource.PropertyPath {
	switch o := o.(type) {
	case []any:
		var paths []resource.PropertyPath
		for i, v := range o {
			paths = append(paths, resource.PropertyPath{i})
			for _, p := range valuePaths(v) {
				paths = append(paths, append(resource.PropertyPath{i}, p...))
			}
		}
		return paths
	case map[string]any:
		isSecure, _ := isSecureValue(o)
		if isSecure {
			return nil
		}

		var paths []resource.PropertyPath
		for k, v := range o {
			paths = append(paths, resource.PropertyPath{k})
			for _, p := range valuePaths(v) {
				paths = append(paths, append(resource.PropertyPath{k}, p...))
			}
		}
		return paths
	default:
		return nil
	}
}

type gobObject struct {
	value any
}

func init() {
	gob.Register([]any(nil))
	gob.Register(map[string]any(nil))
}

func (o gobObject) MarshalYAML() (any, error) {
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(&o.value); err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(data.Bytes())
	return b64, nil
}

func (o *gobObject) UnmarshalYAML(unmarshal func(v any) error) error {
	var b64 string
	if err := unmarshal(&b64); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(data)).Decode(&o.value)
}

func TestRepr(t *testing.T) {
	t.Parallel()

	type expectedValue struct {
		Value        Value     `yaml:"value"`
		String       string    `yaml:"string"`
		Redacted     string    `yaml:"redacted"`
		Object       gobObject `yaml:"object"`
		Secure       bool      `yaml:"secure"`
		IsObject     bool      `yaml:"isObject"`
		SecureValues []string  `yaml:"secureValues,omitempty"`
	}

	type expectedRepr struct {
		Decrypt map[string]string        `yaml:decrypt"`
		Paths   map[string]expectedValue `yaml:"paths"`
	}

	isAccept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	root := filepath.Join("testdata", "repr")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".yaml")
		if !ok || strings.HasSuffix(id, ".expected") {
			continue
		}
		basePath := filepath.Join(root, id)

		t.Run(id, func(t *testing.T) {
			expectedYAMLBytes, err := os.ReadFile(basePath + ".yaml")
			require.NoError(t, err)

			var c Map
			err = yaml.Unmarshal(expectedYAMLBytes, &c)
			require.NoError(t, err)

			yamlBytes, err := yaml.Marshal(c)
			require.NoError(t, err)

			jsonBytes, err := json.Marshal(c)
			require.NoError(t, err)

			decrypted, err := c.Decrypt(NopDecrypter)
			require.NoError(t, err)

			decryptedMap := make(map[string]string)
			for k, v := range decrypted {
				decryptedMap[k.String()] = v
			}

			paths := make(map[string]expectedValue)
			for _, p := range mapPaths(t, c) {
				v, _, _ := c.Get(p, true)

				value, err := v.Value(NopDecrypter)
				require.NoError(t, err)

				redacted, err := v.Value(NewBlindingDecrypter())
				require.NoError(t, err)

				vo, err := v.ToObject()
				require.NoError(t, err)

				secureValues, err := v.SecureValues(NopDecrypter)
				require.NoError(t, err)

				paths[p.String()] = expectedValue{
					Value:        v,
					String:       value,
					Redacted:     redacted,
					Object:       gobObject{value: vo},
					Secure:       v.Secure(),
					IsObject:     v.Object(),
					SecureValues: secureValues,
				}
			}

			actual := expectedRepr{
				Decrypt: decryptedMap,
				Paths:   paths,
			}

			if isAccept {
				expectedBytes, err := yaml.Marshal(actual)
				require.NoError(t, err)

				err = os.WriteFile(basePath+".json", jsonBytes, 0600)
				require.NoError(t, err)

				err = os.WriteFile(basePath+".expected.yaml", expectedBytes, 0600)
				require.NoError(t, err)
			} else {
				expectedJSONBytes, err := os.ReadFile(basePath + ".json")
				require.NoError(t, err)

				var expected expectedRepr
				expectedBytes, err := os.ReadFile(basePath + ".expected.yaml")
				require.NoError(t, err)
				err = yaml.Unmarshal(expectedBytes, &expected)
				require.NoError(t, err)

				assert.Equal(t, expectedYAMLBytes, yamlBytes)
				assert.Equal(t, expectedJSONBytes, jsonBytes)
				assert.Equal(t, expected, actual)
			}
		})
	}
}
