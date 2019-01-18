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

package stack

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/apitype/migrate"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// DeploymentSchemaVersionOldestSupported is the oldest deployment schema that we
	// still support, i.e. we can produce a `deploy.Snapshot` from. This will generally
	// need to be at least one less than the current schema version so that old deployments can
	// be migrated to the current schema.
	DeploymentSchemaVersionOldestSupported = 1
)

var (
	// ErrDeploymentSchemaVersionTooOld is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too old to understand.
	ErrDeploymentSchemaVersionTooOld = fmt.Errorf("this stack's deployment is too old")

	// ErrDeploymentSchemaVersionTooNew is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too new to understand.
	ErrDeploymentSchemaVersionTooNew = fmt.Errorf("this stack's deployment version is too new")
)

// SerializeDeployment serializes an entire snapshot as a deploy record.
func SerializeDeployment(snap *deploy.Snapshot) *apitype.DeploymentV2 {
	contract.Require(snap != nil, "snap")

	// Capture the version information into a manifest.
	manifest := apitype.ManifestV1{
		Time:    snap.Manifest.Time,
		Magic:   snap.Manifest.Magic,
		Version: snap.Manifest.Version,
	}
	for _, plug := range snap.Manifest.Plugins {
		var version string
		if plug.Version != nil {
			version = plug.Version.String()
		}
		manifest.Plugins = append(manifest.Plugins, apitype.PluginInfoV1{
			Name:    plug.Name,
			Path:    plug.Path,
			Type:    plug.Kind,
			Version: version,
		})
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resources []apitype.ResourceV2
	for _, res := range snap.Resources {
		resources = append(resources, SerializeResource(res))
	}

	var operations []apitype.OperationV1
	for _, op := range snap.PendingOperations {
		operations = append(operations, SerializeOperation(op))
	}

	return &apitype.DeploymentV2{
		Manifest:          manifest,
		Resources:         resources,
		PendingOperations: operations,
	}
}

// DeserializeUntypedDeployment deserializes an untyped deployment and produces a `deploy.Snapshot`
// from it. DeserializeDeployment will return an error if the untyped deployment's version is
// not within the range `DeploymentSchemaVersionCurrent` and `DeploymentSchemaVersionOldestSupported`.
func DeserializeUntypedDeployment(deployment *apitype.UntypedDeployment) (*deploy.Snapshot, error) {
	contract.Require(deployment != nil, "deployment")
	switch {
	case deployment.Version > apitype.DeploymentSchemaVersionCurrent:
		return nil, ErrDeploymentSchemaVersionTooNew
	case deployment.Version < DeploymentSchemaVersionOldestSupported:
		return nil, ErrDeploymentSchemaVersionTooOld
	}

	var v2deployment apitype.DeploymentV2
	switch deployment.Version {
	case 1:
		var v1deployment apitype.DeploymentV1
		if err := json.Unmarshal([]byte(deployment.Deployment), &v1deployment); err != nil {
			return nil, err
		}

		v2deployment = migrate.UpToDeploymentV2(v1deployment)
	case 2:
		if err := json.Unmarshal([]byte(deployment.Deployment), &v2deployment); err != nil {
			return nil, err
		}
	default:
		contract.Failf("unrecognized version: %d", deployment.Version)
	}

	return DeserializeDeploymentV2(v2deployment)
}

// DeserializeDeploymentV2 deserializes a typed DeploymentV2 into a `deploy.Snapshot`.
func DeserializeDeploymentV2(deployment apitype.DeploymentV2) (*deploy.Snapshot, error) {
	// Unpack the versions.
	manifest := deploy.Manifest{
		Time:    deployment.Manifest.Time,
		Magic:   deployment.Manifest.Magic,
		Version: deployment.Manifest.Version,
	}
	for _, plug := range deployment.Manifest.Plugins {
		var version *semver.Version
		if v := plug.Version; v != "" {
			sv, err := semver.ParseTolerant(v)
			if err != nil {
				return nil, err
			}
			version = &sv
		}
		manifest.Plugins = append(manifest.Plugins, workspace.PluginInfo{
			Name:    plug.Name,
			Kind:    plug.Type,
			Version: version,
		})
	}

	// For every serialized resource vertex, create a ResourceDeployment out of it.
	var resources []*resource.State
	for _, res := range deployment.Resources {
		desres, err := DeserializeResource(res)
		if err != nil {
			return nil, err
		}
		resources = append(resources, desres)
	}

	var ops []resource.Operation
	for _, op := range deployment.PendingOperations {
		desop, err := DeserializeOperation(op)
		if err != nil {
			return nil, err
		}
		ops = append(ops, desop)
	}

	return deploy.NewSnapshot(manifest, resources, ops), nil
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(res *resource.State) apitype.ResourceV2 {
	contract.Assert(res != nil)
	contract.Assertf(string(res.URN) != "", "Unexpected empty resource resource.URN")

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs; inp != nil {
		inputs = SerializeProperties(inp)
	}
	var outputs map[string]interface{}
	if outp := res.Outputs; outp != nil {
		outputs = SerializeProperties(outp)
	}

	return apitype.ResourceV2{
		URN:          res.URN,
		Custom:       res.Custom,
		Delete:       res.Delete,
		ID:           res.ID,
		Type:         res.Type,
		Parent:       res.Parent,
		Inputs:       inputs,
		Outputs:      outputs,
		Protect:      res.Protect,
		External:     res.External,
		Dependencies: res.Dependencies,
		InitErrors:   res.InitErrors,
		Provider:     res.Provider,
	}
}

func SerializeOperation(op resource.Operation) apitype.OperationV1 {
	res := SerializeResource(op.Resource)
	return apitype.OperationV1{
		Resource: res,
		Type:     apitype.OperationType(op.Type),
	}
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props resource.PropertyMap) map[string]interface{} {
	dst := make(map[string]interface{})
	for _, k := range props.StableKeys() {
		if v := SerializePropertyValue(props[k]); v != nil {
			dst[string(k)] = v
		}
	}
	return dst
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(prop resource.PropertyValue) interface{} {
	// Skip nulls and "outputs"; the former needn't be serialized, and the latter happens if there is an output
	// that hasn't materialized (either because we're serializing inputs or the provider didn't give us the value).
	if prop.IsComputed() || !prop.HasValue() {
		return nil
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		srcarr := prop.ArrayValue()
		dstarr := make([]interface{}, len(srcarr))
		for i, elem := range prop.ArrayValue() {
			dstarr[i] = SerializePropertyValue(elem)
		}
		return dstarr
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeProperties(prop.ObjectValue())
	}

	// For assets, we need to serialize them a little carefully, so we can recover them afterwards.
	if prop.IsAsset() {
		return prop.AssetValue().Serialize()
	} else if prop.IsArchive() {
		return prop.ArchiveValue().Serialize()
	}

	// All others are returned as-is.
	return prop.V
}

// DeserializeResource turns a serialized resource back into its usual form.
func DeserializeResource(res apitype.ResourceV2) (*resource.State, error) {
	// Deserialize the resource properties, if they exist.
	inputs, err := DeserializeProperties(res.Inputs)
	if err != nil {
		return nil, err
	}
	outputs, err := DeserializeProperties(res.Outputs)
	if err != nil {
		return nil, err
	}

	// TODO(pdg): deserialize property dependencies
	propertyDependencies := make(map[resource.PropertyKey][]resource.URN)
	for pk := range inputs {
		propertyDependencies[pk] = res.Dependencies
	}

	return resource.NewState(
		res.Type, res.URN, res.Custom, res.Delete, res.ID,
		inputs, outputs, res.Parent, res.Protect, res.External, res.Dependencies, res.InitErrors, res.Provider,
		propertyDependencies), nil
}

func DeserializeOperation(op apitype.OperationV1) (resource.Operation, error) {
	res, err := DeserializeResource(op.Resource)
	if err != nil {
		return resource.Operation{}, err
	}
	return resource.NewOperation(res, resource.OperationType(op.Type)), nil
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]interface{}) (resource.PropertyMap, error) {
	result := make(resource.PropertyMap)
	for k, prop := range props {
		desprop, err := DeserializePropertyValue(prop)
		if err != nil {
			return nil, err
		}
		result[resource.PropertyKey(k)] = desprop
	}
	return result, nil
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v interface{}) (resource.PropertyValue, error) {
	if v != nil {
		switch w := v.(type) {
		case bool:
			return resource.NewBoolProperty(w), nil
		case float64:
			return resource.NewNumberProperty(w), nil
		case string:
			return resource.NewStringProperty(w), nil
		case []interface{}:
			var arr []resource.PropertyValue
			for _, elem := range w {
				ev, err := DeserializePropertyValue(elem)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				arr = append(arr, ev)
			}
			return resource.NewArrayProperty(arr), nil
		case map[string]interface{}:
			obj, err := DeserializeProperties(w)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			// This could be an asset or archive; if so, recover its type.
			objmap := obj.Mappable()
			asset, isasset, err := resource.DeserializeAsset(objmap)
			if err != nil {
				return resource.PropertyValue{}, err
			} else if isasset {
				return resource.NewAssetProperty(asset), nil
			}
			archive, isarchive, err := resource.DeserializeArchive(objmap)
			if err != nil {
				return resource.PropertyValue{}, err
			} else if isarchive {
				return resource.NewArchiveProperty(archive), nil
			}
			// Otherwise, it's just a weakly typed object map.
			return resource.NewObjectProperty(obj), nil
		default:
			contract.Failf("Unrecognized property type: %v", reflect.ValueOf(v))
		}
	}

	return resource.NewNullProperty(), nil
}
