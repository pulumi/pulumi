// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
)

// JSONToPropertyMap converts a JSON-like map to a Pulumi PropertyMap.
// Handles special Pulumi types encoded in JSON.
func JSONToPropertyMap(jsonMap map[string]any) (resource.PropertyMap, error) {
	if jsonMap == nil {
		return resource.PropertyMap{}, nil
	}

	result := make(resource.PropertyMap)
	for k, v := range jsonMap {
		pv, err := jsonToPropertyValue(v)
		if err != nil {
			return nil, fmt.Errorf("converting property %q: %w", k, err)
		}
		result[resource.PropertyKey(k)] = pv
	}
	return result, nil
}

// jsonToPropertyValue converts a JSON value to a PropertyValue.
func jsonToPropertyValue(v any) (resource.PropertyValue, error) {
	if v == nil {
		return resource.NewNullProperty(), nil
	}

	switch val := v.(type) {
	case bool:
		return resource.NewBoolProperty(val), nil
	case float64:
		return resource.NewNumberProperty(val), nil
	case string:
		return resource.NewStringProperty(val), nil
	case []any:
		arr := make([]resource.PropertyValue, len(val))
		for i, item := range val {
			pv, err := jsonToPropertyValue(item)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("converting array item %d: %w", i, err)
			}
			arr[i] = pv
		}
		return resource.NewArrayProperty(arr), nil
	case map[string]any:
		// Check for special Pulumi types
		if sig, ok := val["4dabf18193072939515e22adb298388d"].(string); ok {
			switch sig {
			case "0def7320c3a5731c473e5ecbe6d01bc7":
				// Asset
				return parseAsset(val)
			case "c44067f5952c0a294b673a41bacd8c17":
				// Archive
				return parseArchive(val)
			case "1b47061264138c4ac30d75fd1eb44270":
				// Secret
				return parseSecret(val)
			case "cfe97e649c90f5f7c0d6c9c3b0c4e3e6":
				// Resource reference
				return parseResourceReference(val)
			}
		}

		// Regular object
		obj := make(resource.PropertyMap)
		for k, v := range val {
			pv, err := jsonToPropertyValue(v)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("converting object property %q: %w", k, err)
			}
			obj[resource.PropertyKey(k)] = pv
		}
		return resource.NewObjectProperty(obj), nil
	default:
		return resource.PropertyValue{}, fmt.Errorf("unsupported type: %T", v)
	}
}

// PropertyMapToJSON converts a Pulumi PropertyMap to a JSON-like map.
// Handles special Pulumi types by encoding them in JSON.
func PropertyMapToJSON(props resource.PropertyMap) (map[string]any, error) {
	if props == nil {
		return map[string]any{}, nil
	}

	result := make(map[string]any)
	for k, v := range props {
		jsonVal, err := propertyValueToJSON(v)
		if err != nil {
			return nil, fmt.Errorf("converting property %q: %w", k, err)
		}
		result[string(k)] = jsonVal
	}
	return result, nil
}

// propertyValueToJSON converts a PropertyValue to a JSON value.
func propertyValueToJSON(v resource.PropertyValue) (any, error) {
	if v.IsNull() {
		return nil, nil
	}

	if v.IsBool() {
		return v.BoolValue(), nil
	}
	if v.IsNumber() {
		return v.NumberValue(), nil
	}
	if v.IsString() {
		return v.StringValue(), nil
	}
	if v.IsArray() {
		arr := v.ArrayValue()
		result := make([]any, len(arr))
		for i, item := range arr {
			jsonVal, err := propertyValueToJSON(item)
			if err != nil {
				return nil, fmt.Errorf("converting array item %d: %w", i, err)
			}
			result[i] = jsonVal
		}
		return result, nil
	}
	if v.IsAsset() {
		return assetToJSON(v.AssetValue()), nil
	}
	if v.IsArchive() {
		return archiveToJSON(v.ArchiveValue()), nil
	}
	if v.IsSecret() {
		secretVal, err := propertyValueToJSON(v.SecretValue().Element)
		if err != nil {
			return nil, fmt.Errorf("converting secret value: %w", err)
		}
		return map[string]any{
			"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
			"value": secretVal,
		}, nil
	}
	if v.IsResourceReference() {
		ref := v.ResourceReferenceValue()
		return map[string]any{
			"4dabf18193072939515e22adb298388d": "cfe97e649c90f5f7c0d6c9c3b0c4e3e6",
			"urn": string(ref.URN),
			"id":  ref.ID.StringValue(),
		}, nil
	}
	if v.IsOutput() {
		output := v.OutputValue()
		elemVal, err := propertyValueToJSON(output.Element)
		if err != nil {
			return nil, fmt.Errorf("converting output value: %w", err)
		}
		deps := make([]string, len(output.Dependencies))
		for i, dep := range output.Dependencies {
			deps[i] = string(dep)
		}
		return map[string]any{
			"4dabf18193072939515e22adb298388d": "output",
			"value":        elemVal,
			"known":        output.Known,
			"secret":       output.Secret,
			"dependencies": deps,
		}, nil
	}
	if v.IsComputed() {
		return map[string]any{
			"4dabf18193072939515e22adb298388d": "computed",
		}, nil
	}
	if v.IsObject() {
		obj := v.ObjectValue()
		result := make(map[string]any)
		for k, v := range obj {
			jsonVal, err := propertyValueToJSON(v)
			if err != nil {
				return nil, fmt.Errorf("converting object property %q: %w", k, err)
			}
			result[string(k)] = jsonVal
		}
		return result, nil
	}

	return nil, fmt.Errorf("unsupported property type")
}

// Helper functions for assets and archives

func parseAsset(m map[string]any) (resource.PropertyValue, error) {
	if text, ok := m["text"].(string); ok {
		asset, err := asset.FromText(text)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewAssetProperty(asset), nil
	}
	if path, ok := m["path"].(string); ok {
		asset, err := asset.FromPath(path)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewAssetProperty(asset), nil
	}
	if uri, ok := m["uri"].(string); ok {
		asset, err := asset.FromURI(uri)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewAssetProperty(asset), nil
	}
	return resource.PropertyValue{}, fmt.Errorf("invalid asset: missing text, path, or uri")
}

func parseArchive(m map[string]any) (resource.PropertyValue, error) {
	if path, ok := m["path"].(string); ok {
		arch, err := archive.FromPath(path)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewArchiveProperty(arch), nil
	}
	if uri, ok := m["uri"].(string); ok {
		arch, err := archive.FromURI(uri)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewArchiveProperty(arch), nil
	}
	if assets, ok := m["assets"].(map[string]any); ok {
		assetMap := make(map[string]any)
		for k, v := range assets {
			pv, err := jsonToPropertyValue(v)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			if pv.IsAsset() {
				assetMap[k] = pv.AssetValue()
			} else if pv.IsArchive() {
				assetMap[k] = pv.ArchiveValue()
			} else {
				return resource.PropertyValue{}, fmt.Errorf("archive assets must be assets or archives")
			}
		}
		arch, err := archive.FromAssets(assetMap)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewArchiveProperty(arch), nil
	}
	return resource.PropertyValue{}, fmt.Errorf("invalid archive: missing path, uri, or assets")
}

func parseSecret(m map[string]any) (resource.PropertyValue, error) {
	val, ok := m["value"]
	if !ok {
		return resource.PropertyValue{}, fmt.Errorf("secret missing value")
	}
	pv, err := jsonToPropertyValue(val)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	return resource.MakeSecret(pv), nil
}

func parseResourceReference(m map[string]any) (resource.PropertyValue, error) {
	urnStr, ok := m["urn"].(string)
	if !ok {
		return resource.PropertyValue{}, fmt.Errorf("resource reference missing urn")
	}
	idVal := m["id"]
	var id resource.PropertyValue
	if idVal != nil {
		var err error
		id, err = jsonToPropertyValue(idVal)
		if err != nil {
			return resource.PropertyValue{}, err
		}
	} else {
		id = resource.NewStringProperty("")
	}

	ref := resource.ResourceReference{
		URN: resource.URN(urnStr),
		ID:  id,
	}
	return resource.NewResourceReferenceProperty(ref), nil
}

func assetToJSON(a *asset.Asset) map[string]any {
	result := map[string]any{
		"4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
	}
	if a.IsText() {
		result["text"] = a.Text
		result["hash"] = a.Hash
	} else if a.IsPath() {
		result["path"] = a.Path
		result["hash"] = a.Hash
	} else if a.IsURI() {
		result["uri"] = a.URI
		result["hash"] = a.Hash
	}
	return result
}

func archiveToJSON(a *archive.Archive) map[string]any {
	result := map[string]any{
		"4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
	}
	if a.IsPath() {
		result["path"] = a.Path
		result["hash"] = a.Hash
	} else if a.IsURI() {
		result["uri"] = a.URI
		result["hash"] = a.Hash
	} else if a.IsAssets() {
		assets := make(map[string]any)
		for k, v := range a.Assets {
			switch val := v.(type) {
			case *asset.Asset:
				assets[k] = assetToJSON(val)
			case *archive.Archive:
				assets[k] = archiveToJSON(val)
			}
		}
		result["assets"] = assets
		result["hash"] = a.Hash
	}
	return result
}

// MarshalPropertyMap marshals a PropertyMap to JSON bytes.
func MarshalPropertyMap(props resource.PropertyMap) ([]byte, error) {
	m, err := PropertyMapToJSON(props)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// UnmarshalPropertyMap unmarshals JSON bytes to a PropertyMap.
func UnmarshalPropertyMap(data []byte) (resource.PropertyMap, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return JSONToPropertyMap(m)
}

// RandomSeedToBytes converts a base64-encoded random seed to bytes.
func RandomSeedToBytes(seed string) ([]byte, error) {
	if seed == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(seed)
}

// RandomSeedToString converts random seed bytes to base64.
func RandomSeedToString(seed []byte) string {
	if seed == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(seed)
}
