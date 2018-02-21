// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package stack

import (
	"reflect"
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Deployment is a serializable, flattened LumiGL graph structure, representing a deploy.   It is similar
// to the actual Snapshot structure, except that it flattens and rearranges a few data structures for serializability.
// Over time, we also expect this to gather more information about deploys themselves.
type Deployment struct {
	Manifest  Manifest   `json:"manifest" yaml:"manifest"`                       // the deployment's manifest.
	Resources []Resource `json:"resources,omitempty" yaml:"resources,omitempty"` // an array of resources.
}

// Manifest captures meta-information about this checkpoint file, such as versions of binaries, etc.
type Manifest struct {
	Time    time.Time    `json:"time" yaml:"time"`                           // the time of the deploy.
	Magic   string       `json:"magic" yaml:"magic"`                         // a magic cookie.
	Version string       `json:"version" yaml:"version"`                     // the version of the Pulumi CLI.
	Plugins []PluginInfo `json:"plugins,omitempty" yaml:"plugins,omitempty"` // the plugin binary versions.
}

// PluginInfo captures the version and information about a plugin.
type PluginInfo struct {
	Name    string               `json:"name" yaml:"name"`
	Type    workspace.PluginKind `json:"type" yaml:"type"`
	Version string               `json:"version" yaml:"version"`
}

// Resource is a serializable vertex within a LumiGL graph, specifically for resource snapshots.
// nolint: lll
type Resource struct {
	URN          resource.URN           `json:"urn" yaml:"urn"`                               // the URN for this resource.
	Custom       bool                   `json:"custom" yaml:"custom"`                         // if this is a custom resource managed by a plugin.
	Delete       bool                   `json:"delete,omitempty" yaml:"delete,omitempty"`     // if this should be deleted during the next update.
	ID           resource.ID            `json:"id,omitempty" yaml:"id,omitempty"`             // the provider ID for this resource, if any.
	Type         tokens.Type            `json:"type" yaml:"type"`                             // this resource's full type token.
	Inputs       map[string]interface{} `json:"inputs,omitempty" yaml:"inputs,omitempty"`     // the input properties from the provider (or the program for ressources with defaults).
	Defaults     map[string]interface{} `json:"defaults,omitempty" yaml:"defaults,omitempty"` // the default property values from the provider (DEPRECATED, see #637).
	Outputs      map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`   // the output properties from the resource provider.
	Parent       resource.URN           `json:"parent,omitempty" yaml:"parent,omitempty"`     // an optional parent URN if this is a child resource.
	Protect      bool                   `json:"protect,omitempty" yaml:"protect,omitempty"`   // true if this resource is protected, and cannot be deleted.
	Dependencies []resource.URN         `json:"dependencies" yaml:"dependencies"`             // the dependency edges for this resource
}

// SerializeDeployment serializes an entire snapshot as a deploy record.
func SerializeDeployment(snap *deploy.Snapshot) *Deployment {
	// Capture the version information into a manifest.
	manifest := Manifest{
		Time:    snap.Manifest.Time,
		Magic:   snap.Manifest.Magic,
		Version: snap.Manifest.Version,
	}
	for _, plug := range snap.Manifest.Plugins {
		var version string
		if plug.Version != nil {
			version = plug.Version.String()
		}
		manifest.Plugins = append(manifest.Plugins, PluginInfo{
			Name:    plug.Name,
			Type:    plug.Kind,
			Version: version,
		})
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resources []Resource
	for _, res := range snap.Resources {
		resources = append(resources, SerializeResource(res))
	}

	return &Deployment{
		Manifest:  manifest,
		Resources: resources,
	}
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(res *resource.State) Resource {
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

	return Resource{
		URN:          res.URN,
		Custom:       res.Custom,
		Delete:       res.Delete,
		ID:           res.ID,
		Type:         res.Type,
		Parent:       res.Parent,
		Inputs:       inputs,
		Outputs:      outputs,
		Protect:      res.Protect,
		Dependencies: res.Dependencies,
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
func DeserializeResource(res Resource) (*resource.State, error) {
	// Deserialize the resource properties, if they exist.
	inputs, err := DeserializeProperties(res.Inputs)
	if err != nil {
		return nil, err
	}
	defaults, err := DeserializeProperties(res.Defaults)
	if err != nil {
		return nil, err
	}
	outputs, err := DeserializeProperties(res.Outputs)
	if err != nil {
		return nil, err
	}

	// If this is an old checkpoint that still had defaults, merge the inputs into the defaults.
	//
	// NOTE: we will remove support for defaults entirely in the future. See #637.
	if inputs != nil && defaults != nil {
		inputs = defaults.Merge(inputs)
	}

	return resource.NewState(
		res.Type, res.URN, res.Custom, res.Delete, res.ID, inputs, outputs, res.Parent, res.Protect, res.Dependencies), nil
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
