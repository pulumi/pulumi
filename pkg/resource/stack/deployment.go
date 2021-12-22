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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack/events"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype/migrate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	// DeploymentSchemaVersionOldestSupported is the oldest deployment schema that we
	// still support, i.e. we can produce a `deploy.Snapshot` from. This will generally
	// need to be at least one less than the current schema version so that old deployments can
	// be migrated to the current schema.
	DeploymentSchemaVersionOldestSupported = 1

	// computedValue is a magic number we emit for a value of a resource.Property value
	// whenever we need to serialize a resource.Computed. (Since the real/actual value
	// is not known.) This allows us to persist engine events and resource states that
	// indicate a value will changed... but is unknown what it will change to.
	computedValuePlaceholder = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
)

var (
	// ErrDeploymentSchemaVersionTooOld is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too old to understand.
	ErrDeploymentSchemaVersionTooOld = fmt.Errorf("this stack's deployment is too old")

	// ErrDeploymentSchemaVersionTooNew is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too new to understand.
	ErrDeploymentSchemaVersionTooNew = fmt.Errorf("this stack's deployment version is too new")
)

var deploymentSchema *jsonschema.Schema
var resourceSchema *jsonschema.Schema
var propertyValueSchema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(s string) (io.ReadCloser, error) {
		var schema string
		switch s {
		case apitype.DeploymentSchemaID:
			schema = apitype.DeploymentSchema()
		case apitype.ResourceSchemaID:
			schema = apitype.ResourceSchema()
		case apitype.PropertyValueSchemaID:
			schema = apitype.PropertyValueSchema()
		default:
			return jsonschema.LoadURL(s)
		}
		return ioutil.NopCloser(strings.NewReader(schema)), nil
	}
	deploymentSchema = compiler.MustCompile(apitype.DeploymentSchemaID)
	resourceSchema = compiler.MustCompile(apitype.ResourceSchemaID)
	propertyValueSchema = compiler.MustCompile(apitype.PropertyValueSchemaID)
}

// ValidateUntypedDeployment validates a deployment against the Deployment JSON schema.
func ValidateUntypedDeployment(deployment *apitype.UntypedDeployment) error {
	bytes, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	var raw interface{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	return deploymentSchema.Validate(raw)
}

// SerializeDeployment serializes an entire snapshot as a deploy record.
func SerializeDeployment(snap *deploy.Snapshot, sm secrets.Manager, showSecrets bool) (*apitype.DeploymentV3, error) {
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

	// If a specific secrets manager was not provided, use the one in the snapshot, if present.
	if sm == nil {
		sm = snap.SecretsManager
	}

	var enc config.Encrypter
	if sm != nil {
		e, err := sm.Encrypter()
		if err != nil {
			return nil, fmt.Errorf("getting encrypter for deployment: %w", err)
		}
		enc = e
	} else {
		enc = config.NewPanicCrypter()
	}

	// Serialize all vertices and only include a vertex section if non-empty.
	var resources []apitype.ResourceV3
	for _, res := range snap.Resources {
		sres, err := SerializeResource(res, enc, showSecrets)
		if err != nil {
			return nil, fmt.Errorf("serializing resources: %w", err)
		}
		resources = append(resources, sres)
	}

	var operations []apitype.OperationV2
	for _, op := range snap.PendingOperations {
		sop, err := SerializeOperation(op, enc, showSecrets)
		if err != nil {
			return nil, err
		}
		operations = append(operations, sop)
	}

	var secretsProvider *apitype.SecretsProvidersV1
	if sm != nil {
		secretsProvider = &apitype.SecretsProvidersV1{
			Type: sm.Type(),
		}
		if state := sm.State(); state != nil {
			rm, err := json.Marshal(state)
			if err != nil {
				return nil, err
			}
			secretsProvider.State = rm
		}
	}

	return &apitype.DeploymentV3{
		Manifest:          manifest,
		Resources:         resources,
		SecretsProviders:  secretsProvider,
		PendingOperations: operations,
	}, nil
}

// DeserializeUntypedDeployment deserializes an untyped deployment and produces a `deploy.Snapshot`
// from it. DeserializeDeployment will return an error if the untyped deployment's version is
// not within the range `DeploymentSchemaVersionCurrent` and `DeploymentSchemaVersionOldestSupported`.
func DeserializeUntypedDeployment(
	deployment *apitype.UntypedDeployment, secretsProv SecretsProvider) (*deploy.Snapshot, error) {

	contract.Require(deployment != nil, "deployment")
	switch {
	case deployment.Version > apitype.DeploymentSchemaVersionCurrent:
		return nil, ErrDeploymentSchemaVersionTooNew
	case deployment.Version < DeploymentSchemaVersionOldestSupported:
		return nil, ErrDeploymentSchemaVersionTooOld
	}

	var v3deployment apitype.DeploymentV3
	switch deployment.Version {
	case 1:
		var v1deployment apitype.DeploymentV1
		if err := json.Unmarshal([]byte(deployment.Deployment), &v1deployment); err != nil {
			return nil, err
		}
		v2deployment := migrate.UpToDeploymentV2(v1deployment)
		v3deployment = migrate.UpToDeploymentV3(v2deployment)
	case 2:
		var v2deployment apitype.DeploymentV2
		if err := json.Unmarshal([]byte(deployment.Deployment), &v2deployment); err != nil {
			return nil, err
		}
		v3deployment = migrate.UpToDeploymentV3(v2deployment)
	case 3:
		if err := json.Unmarshal([]byte(deployment.Deployment), &v3deployment); err != nil {
			return nil, err
		}
	default:
		contract.Failf("unrecognized version: %d", deployment.Version)
	}

	return DeserializeDeploymentV3(v3deployment, secretsProv)
}

// DeserializeDeploymentV3 deserializes a typed DeploymentV3 into a `deploy.Snapshot`.
func DeserializeDeploymentV3(deployment apitype.DeploymentV3, secretsProv SecretsProvider) (*deploy.Snapshot, error) {
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

	var secretsManager secrets.Manager
	if deployment.SecretsProviders != nil && deployment.SecretsProviders.Type != "" {
		if secretsProv == nil {
			return nil, errors.New("deployment uses a SecretsProvider but no SecretsProvider was provided")
		}

		sm, err := secretsProv.OfType(deployment.SecretsProviders.Type, deployment.SecretsProviders.State)
		if err != nil {
			return nil, err
		}
		secretsManager = sm
	}

	var dec config.Decrypter
	var enc config.Encrypter
	if secretsManager == nil {
		dec = config.NewPanicCrypter()
		enc = config.NewPanicCrypter()
	} else {
		d, err := secretsManager.Decrypter()
		if err != nil {
			return nil, err
		}
		dec = d

		e, err := secretsManager.Encrypter()
		if err != nil {
			return nil, err
		}
		enc = e
	}

	// For every serialized resource vertex, create a ResourceDeployment out of it.
	var resources []*resource.State
	for _, res := range deployment.Resources {
		desres, err := DeserializeResource(res, dec, enc)
		if err != nil {
			return nil, err
		}
		resources = append(resources, desres)
	}

	var ops []resource.Operation
	for _, op := range deployment.PendingOperations {
		desop, err := DeserializeOperation(op, dec, enc)
		if err != nil {
			return nil, err
		}
		ops = append(ops, desop)
	}

	return deploy.NewSnapshot(manifest, secretsManager, resources, ops), nil
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(res *resource.State, enc config.Encrypter, showSecrets bool) (apitype.ResourceV3, error) {
	return events.SerializeResource(res, enc, showSecrets)
}

func SerializeOperation(op resource.Operation, enc config.Encrypter, showSecrets bool) (apitype.OperationV2, error) {
	res, err := SerializeResource(op.Resource, enc, showSecrets)
	if err != nil {
		return apitype.OperationV2{}, fmt.Errorf("serializing resource: %w", err)
	}
	return apitype.OperationV2{
		Resource: res,
		Type:     apitype.OperationType(op.Type),
	}, nil
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props resource.PropertyMap, enc config.Encrypter,
	showSecrets bool) (map[string]interface{}, error) {

	return events.SerializeProperties(props, enc, showSecrets)
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(prop resource.PropertyValue, enc config.Encrypter,
	showSecrets bool) (interface{}, error) {

	return events.SerializePropertyValue(prop, enc, showSecrets)
}

// DeserializeResource turns a serialized resource back into its usual form.
func DeserializeResource(res apitype.ResourceV3, dec config.Decrypter, enc config.Encrypter) (*resource.State, error) {
	return events.DeserializeResource(res, dec, enc)
}

func DeserializeOperation(op apitype.OperationV2, dec config.Decrypter,
	enc config.Encrypter) (resource.Operation, error) {
	res, err := DeserializeResource(op.Resource, dec, enc)
	if err != nil {
		return resource.Operation{}, err
	}
	return resource.NewOperation(res, resource.OperationType(op.Type)), nil
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]interface{}, dec config.Decrypter,
	enc config.Encrypter) (resource.PropertyMap, error) {

	return events.DeserializeProperties(props, dec, enc)
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v interface{}, dec config.Decrypter,
	enc config.Encrypter) (resource.PropertyValue, error) {

	return events.DeserializePropertyValue(v, dec, enc)
}
