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

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype/migrate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	// DeploymentSchemaVersionOldestSupported is the oldest deployment schema that we
	// still support, i.e. we can produce a `deploy.Snapshot` from. This will generally
	// need to be at least one less than the current schema version so that old deployments can
	// be migrated to the current schema.
	DeploymentSchemaVersionOldestSupported = 1

	// DeploymentSchemaVersionLatest is the latest version of the `Deployment` schema.
	// Any deployments newer than this version will be rejected.
	DeploymentSchemaVersionLatest = 4

	// computedValue is a magic number we emit for a value of a resource.Property value
	// whenever we need to serialize a resource.Computed. (Since the real/actual value
	// is not known.) This allows us to persist engine events and resource states that
	// indicate a value will changed... but is unknown what it will change to.
	computedValuePlaceholder = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

	// Feature names for deployment features.
	refreshBeforeUpdateFeature = "refreshBeforeUpdate"
	viewsFeature               = "views"
	hooksFeature               = "hooks"
	taintFeature               = "taint"
)

var (
	// ErrDeploymentSchemaVersionTooOld is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too old to understand.
	ErrDeploymentSchemaVersionTooOld = errors.New("this stack's deployment is too old")

	// ErrDeploymentSchemaVersionTooNew is returned from `DeserializeDeployment` if the
	// untyped deployment being deserialized is too new to understand.
	ErrDeploymentSchemaVersionTooNew = errors.New("this stack's deployment version is too new")
)

// ErrDeploymentUnsupportedFeatures is returned from `DeserializeDeployment` if the
// untyped deployment being deserialized uses one or more features that are not supported.
type ErrDeploymentUnsupportedFeatures struct {
	// The unsupported features.
	Features []string
}

func (e *ErrDeploymentUnsupportedFeatures) Error() string {
	return "this stack's deployment needs support for the following features: " + strings.Join(e.Features, ", ")
}

var (
	deploymentSchema    *jsonschema.Schema
	propertyValueSchema *jsonschema.Schema
)

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
		return io.NopCloser(strings.NewReader(schema)), nil
	}
	deploymentSchema = compiler.MustCompile(apitype.DeploymentSchemaID)
	propertyValueSchema = compiler.MustCompile(apitype.PropertyValueSchemaID)
}

// supportedFeatures is a map of features that are currently supported.
// Any features not in this map will be rejected.
var supportedFeatures = map[string]bool{
	refreshBeforeUpdateFeature: true,
	viewsFeature:               true,
	hooksFeature:               true,
	taintFeature:               true,
}

// validateSupportedFeatures validates that the features used in a deployment are supported.
func validateSupportedFeatures(features []string) error {
	var unsupported []string
	for _, feature := range features {
		if !supportedFeatures[feature] {
			unsupported = append(unsupported, feature)
		}
	}
	if len(unsupported) > 0 {
		return &ErrDeploymentUnsupportedFeatures{Features: unsupported}
	}
	return nil
}

// applyFeatures applies the features used by a resource to the feature map.
func applyFeatures(res apitype.ResourceV3, features map[string]bool) {
	if res.RefreshBeforeUpdate {
		features[refreshBeforeUpdateFeature] = true
	}
	if res.ViewOf != "" {
		features[viewsFeature] = true
	}
	if len(res.ResourceHooks) > 0 {
		features[hooksFeature] = true
	}
	if res.Taint {
		features[taintFeature] = true
	}
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
func SerializeDeployment(ctx context.Context, snap *deploy.Snapshot, showSecrets bool) (*apitype.DeploymentV3, error) {
	serializedDeployment, _, _, err := SerializeDeploymentWithMetadata(ctx, snap, showSecrets)
	return serializedDeployment, err
}

// SerializeDeploymentWithMetadata serializes an entire snapshot as a deploy record returning the deployment, version,
// and features used by the deployment.
func SerializeDeploymentWithMetadata(
	ctx context.Context,
	snap *deploy.Snapshot,
	showSecrets bool,
) (*apitype.DeploymentV3, int, []string, error) {
	contract.Requiref(snap != nil, "snap", "must not be nil")

	// Capture the version information into a manifest.
	manifest := snap.Manifest.Serialize()

	sm := snap.SecretsManager
	var enc config.Encrypter
	var completeBatch CompleteCrypterBatch
	if sm != nil {
		// If the secrets manager supports batching, start the batch operation.
		if batchingSecretsManager, ok := sm.(BatchingSecretsManager); ok {
			enc, completeBatch = batchingSecretsManager.BeginBatchEncryption()
		} else {
			enc = sm.Encrypter()
		}
	} else {
		enc = config.NewPanicCrypter()
	}

	featureMap := map[string]bool{}

	// Serialize all vertices and only include a vertex section if non-empty.
	resources := slice.Prealloc[apitype.ResourceV3](len(snap.Resources))
	for _, res := range snap.Resources {
		sres, err := SerializeResource(ctx, res, enc, showSecrets)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("serializing resources: %w", err)
		}
		applyFeatures(sres, featureMap)
		resources = append(resources, sres)
	}

	operations := slice.Prealloc[apitype.OperationV2](len(snap.PendingOperations))
	for _, op := range snap.PendingOperations {
		sop, err := SerializeOperation(ctx, op, enc, showSecrets)
		if err != nil {
			return nil, 0, nil, err
		}
		operations = append(operations, sop)
	}

	var secretsProvider *apitype.SecretsProvidersV1
	if sm != nil {
		secretsProvider = &apitype.SecretsProvidersV1{
			Type:  sm.Type(),
			State: sm.State(),
		}
	}

	metadata := apitype.SnapshotMetadataV1{}
	if snap.Metadata.IntegrityErrorMetadata != nil {
		metadata.IntegrityErrorMetadata = &apitype.SnapshotIntegrityErrorMetadataV1{
			Version: snap.Metadata.IntegrityErrorMetadata.Version,
			Command: snap.Metadata.IntegrityErrorMetadata.Command,
			Error:   snap.Metadata.IntegrityErrorMetadata.Error,
		}
	}

	if completeBatch != nil { // If we started a batch operation, complete it.
		if err := completeBatch(ctx); err != nil {
			return nil, 0, nil, err
		}
	}

	features := maputil.SortedKeys(featureMap)
	if len(features) == 0 {
		features = nil
	}

	version := apitype.DeploymentSchemaVersionCurrent
	if len(features) > 0 {
		version = DeploymentSchemaVersionLatest
	}

	return &apitype.DeploymentV3{
		Manifest:          manifest,
		Resources:         resources,
		SecretsProviders:  secretsProvider,
		PendingOperations: operations,
		Metadata:          metadata,
	}, version, features, nil
}

// SerializeOptions controls how a deployment is serialized to JSON.
type SerializeOptions struct {
	// ShowSecrets indicates that secrets should be shown in the serialized deployment.
	ShowSecrets bool
	// Pretty indicates that the serialized deployment should be indented and formatted for display to users.
	Pretty bool
}

// SerializeUntypedDeployment serializes a snapshot into an untyped deployment.
func SerializeUntypedDeployment(
	ctx context.Context,
	snap *deploy.Snapshot,
	opts *SerializeOptions,
) (*apitype.UntypedDeployment, error) {
	showSecrets := opts != nil && opts.ShowSecrets
	serializedDeployment, version, features, err := SerializeDeploymentWithMetadata(ctx, snap, showSecrets)
	if err != nil {
		return nil, err
	}

	var jsonDeployment []byte
	if opts != nil && opts.Pretty {
		jsonDeployment, err = encoding.JSON.Marshal(serializedDeployment)
	} else {
		jsonDeployment, err = json.Marshal(serializedDeployment)
	}
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    version,
		Features:   features,
		Deployment: jsonDeployment,
	}, nil
}

// UnmarshalUntypedDeployment unmarshals a raw untyped deployment into an up to date deployment object.
func UnmarshalUntypedDeployment(
	ctx context.Context,
	deployment *apitype.UntypedDeployment,
) (*apitype.DeploymentV3, error) {
	contract.Requiref(deployment != nil, "deployment", "must not be nil")
	switch {
	case deployment.Version > DeploymentSchemaVersionLatest:
		return nil, ErrDeploymentSchemaVersionTooNew
	case deployment.Version < DeploymentSchemaVersionOldestSupported:
		return nil, ErrDeploymentSchemaVersionTooOld
	}

	// Check for unsupported features in deployments with version 4 and above.
	if deployment.Version >= 4 {
		if err := validateSupportedFeatures(deployment.Features); err != nil {
			return nil, err
		}
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
	case 3, 4:
		// Both version 3 and 4 can be unmarshaled into `DeploymentV3`.
		if err := json.Unmarshal([]byte(deployment.Deployment), &v3deployment); err != nil {
			return nil, err
		}
	default:
		contract.Failf("unrecognized version: %d", deployment.Version)
	}

	return &v3deployment, nil
}

// DeserializeUntypedDeployment deserializes an untyped deployment and produces a `deploy.Snapshot`
// from it. DeserializeDeployment will return an error if the untyped deployment's version is
// not within the range `DeploymentSchemaVersionCurrent` and `DeploymentSchemaVersionOldestSupported`.
func DeserializeUntypedDeployment(
	ctx context.Context,
	deployment *apitype.UntypedDeployment,
	secretsProv secrets.Provider,
) (*deploy.Snapshot, error) {
	v3deployment, err := UnmarshalUntypedDeployment(ctx, deployment)
	if err != nil {
		return nil, err
	}
	return DeserializeDeploymentV3(ctx, *v3deployment, secretsProv)
}

// DeserializeDeploymentV3 deserializes a typed DeploymentV3 into a `deploy.Snapshot`.
func DeserializeDeploymentV3(
	ctx context.Context,
	deployment apitype.DeploymentV3,
	secretsProv secrets.Provider,
) (*deploy.Snapshot, error) {
	// Unpack the versions.
	manifest, err := deploy.DeserializeManifest(deployment.Manifest)
	if err != nil {
		return nil, err
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
	var completeBatch CompleteCrypterBatch
	if secretsManager != nil {
		if batchingSecretsManager, ok := secretsManager.(BatchingSecretsManager); ok {
			// If the secrets manager supports batching, start a batch operation.
			dec, completeBatch = batchingSecretsManager.BeginBatchDecryption()
		} else {
			dec = secretsManager.Decrypter()
		}
	} else {
		// We'll attempt to continue without a decrypter, but fail if we encounter encrypted secrets.
		dec = config.NewErrorCrypter("snapshot contains encrypted secrets but no secrets manager could be found")
	}

	// For every serialized resource vertex, create a ResourceDeployment out of it.
	resources := slice.Prealloc[*resource.State](len(deployment.Resources))
	for _, res := range deployment.Resources {
		desres, err := DeserializeResource(res, dec)
		if err != nil {
			return nil, err
		}
		resources = append(resources, desres)
	}

	ops := slice.Prealloc[resource.Operation](len(deployment.PendingOperations))
	for _, op := range deployment.PendingOperations {
		desop, err := DeserializeOperation(op, dec)
		if err != nil {
			return nil, err
		}
		ops = append(ops, desop)
	}

	if completeBatch != nil {
		// If we started a batch operation, complete it.
		if err := completeBatch(ctx); err != nil {
			return nil, err
		}
	}

	metadata := deploy.SnapshotMetadata{}
	if deployment.Metadata.IntegrityErrorMetadata != nil {
		metadata.IntegrityErrorMetadata = &deploy.SnapshotIntegrityErrorMetadata{
			Version: deployment.Metadata.IntegrityErrorMetadata.Version,
			Command: deployment.Metadata.IntegrityErrorMetadata.Command,
			Error:   deployment.Metadata.IntegrityErrorMetadata.Error,
		}
	}

	return deploy.NewSnapshot(*manifest, secretsManager, resources, ops, metadata), nil
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(
	ctx context.Context, res *resource.State, enc config.Encrypter, showSecrets bool,
) (apitype.ResourceV3, error) {
	contract.Requiref(res != nil, "res", "must not be nil")
	contract.Requiref(res.URN != "", "res", "must have a URN")

	res.Lock.Lock()
	defer res.Lock.Unlock()

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs; inp != nil {
		sinp, err := SerializeProperties(ctx, inp, enc, showSecrets)
		if err != nil {
			return apitype.ResourceV3{}, err
		}
		inputs = sinp
	}
	var outputs map[string]interface{}
	if outp := res.Outputs; outp != nil {
		soutp, err := SerializeProperties(ctx, outp, enc, showSecrets)
		if err != nil {
			return apitype.ResourceV3{}, err
		}
		outputs = soutp
	}

	stackTrace := slices.Collect(fxs.Map(res.StackTrace, func(frame resource.StackFrame) apitype.StackFrameV1 {
		return apitype.StackFrameV1{SourcePosition: frame.SourcePosition}
	}))

	v3Resource := apitype.ResourceV3{
		URN:                     res.URN,
		Custom:                  res.Custom,
		Delete:                  res.Delete,
		ID:                      res.ID,
		Type:                    res.Type,
		Parent:                  res.Parent,
		Inputs:                  inputs,
		Outputs:                 outputs,
		Protect:                 res.Protect,
		Taint:                   res.Taint,
		External:                res.External,
		Dependencies:            res.Dependencies,
		InitErrors:              res.InitErrors,
		Provider:                res.Provider,
		PropertyDependencies:    res.PropertyDependencies,
		PendingReplacement:      res.PendingReplacement,
		AdditionalSecretOutputs: res.AdditionalSecretOutputs,
		Aliases:                 res.Aliases,
		ImportID:                res.ImportID,
		RetainOnDelete:          res.RetainOnDelete,
		DeletedWith:             res.DeletedWith,
		Created:                 res.Created,
		Modified:                res.Modified,
		SourcePosition:          res.SourcePosition,
		StackTrace:              stackTrace,
		IgnoreChanges:           res.IgnoreChanges,
		ReplaceOnChanges:        res.ReplaceOnChanges,
		RefreshBeforeUpdate:     res.RefreshBeforeUpdate,
		ViewOf:                  res.ViewOf,
		ResourceHooks:           res.ResourceHooks,
	}

	if res.CustomTimeouts.IsNotEmpty() {
		v3Resource.CustomTimeouts = &res.CustomTimeouts
	}

	return v3Resource, nil
}

// SerializeOperation serializes a resource in a pending state.
func SerializeOperation(
	ctx context.Context, op resource.Operation, enc config.Encrypter, showSecrets bool,
) (apitype.OperationV2, error) {
	res, err := SerializeResource(ctx, op.Resource, enc, showSecrets)
	if err != nil {
		return apitype.OperationV2{}, fmt.Errorf("serializing resource: %w", err)
	}
	return apitype.OperationV2{
		Resource: res,
		Type:     apitype.OperationType(op.Type),
	}, nil
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(ctx context.Context, props resource.PropertyMap, enc config.Encrypter,
	showSecrets bool,
) (map[string]interface{}, error) {
	dst := make(map[string]interface{})
	for _, k := range props.StableKeys() {
		v, err := SerializePropertyValue(ctx, props[k], enc, showSecrets)
		if err != nil {
			return nil, err
		}
		dst[string(k)] = v
	}
	return dst, nil
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(ctx context.Context, prop resource.PropertyValue, enc config.Encrypter,
	showSecrets bool,
) (interface{}, error) {
	// Serialize nulls as nil.
	if prop.IsNull() {
		return nil, nil
	}

	// A computed value marks something that will be determined at a later time. (e.g. the result of
	// a computation that we don't perform during a preview operation.) We serialize a magic constant
	// to record its existence.
	if prop.IsComputed() || prop.IsOutput() {
		return computedValuePlaceholder, nil
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		srcarr := prop.ArrayValue()
		dstarr := make([]interface{}, len(srcarr))
		for i, elem := range prop.ArrayValue() {
			selem, err := SerializePropertyValue(ctx, elem, enc, showSecrets)
			if err != nil {
				return nil, err
			}
			dstarr[i] = selem
		}
		return dstarr, nil
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeProperties(ctx, prop.ObjectValue(), enc, showSecrets)
	}

	// For assets, we need to serialize them a little carefully, so we can recover them afterwards.
	if prop.IsAsset() {
		return prop.AssetValue().Serialize(), nil
	} else if prop.IsArchive() {
		return prop.ArchiveValue().Serialize(), nil
	}

	// We serialize resource references using a map-based representation similar to assets, archives, and secrets.
	if prop.IsResourceReference() {
		ref := prop.ResourceReferenceValue()
		serialized := map[string]interface{}{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            string(ref.URN),
			"packageVersion": ref.PackageVersion,
		}
		if id, hasID := ref.IDString(); hasID {
			serialized["id"] = id
		}
		return serialized, nil
	}

	if prop.IsSecret() {
		// Since we are going to encrypt property value, we can elide encrypting sub-elements. We'll mark them as
		// "secret" so we retain that information when deserializing the overall structure, but there is no
		// need to double encrypt everything.
		value, err := SerializePropertyValue(ctx, prop.SecretValue().Element, config.NopEncrypter, showSecrets)
		if err != nil {
			return nil, err
		}
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encoding serialized property value: %w", err)
		}
		plaintext := string(bytes)

		secret := apitype.SecretV1{
			Sig: resource.SecretSig,
		}

		if showSecrets {
			secret.Plaintext = plaintext
		} else {
			// If the encrypter is a batchEncrypter, use the Enqueue method to asynchronously encrypt the secret value.
			if batchEncrypter, ok := enc.(BatchEncrypter); ok {
				err = batchEncrypter.Enqueue(ctx, prop.SecretValue(), plaintext, &secret)
				if err != nil {
					return nil, fmt.Errorf("enqueuing secret value for encryption: %w", err)
				}
			} else {
				ciphertext, err := enc.EncryptValue(ctx, plaintext)
				if err != nil {
					return nil, fmt.Errorf("failed to encrypt secret value: %w", err)
				}
				secret.Ciphertext = ciphertext
			}
		}

		return &secret, nil
	}

	// All others are returned as-is.
	return prop.V, nil
}

// DeserializeResource turns a serialized resource back into its usual form.
func DeserializeResource(res apitype.ResourceV3, dec config.Decrypter) (*resource.State, error) {
	// Deserialize the resource properties, if they exist.
	inputs, err := DeserializeProperties(res.Inputs, dec)
	if err != nil {
		return nil, err
	}
	outputs, err := DeserializeProperties(res.Outputs, dec)
	if err != nil {
		return nil, err
	}

	if res.URN == "" {
		return nil, errors.New("resource missing required 'urn' field")
	}

	if res.Type == "" {
		return nil, fmt.Errorf("resource '%s' missing required 'type' field", res.URN)
	}

	if !res.Custom && res.ID != "" {
		return nil, fmt.Errorf("resource '%s' has 'custom' false but non-empty ID", res.URN)
	}

	stackTrace := slices.Collect(fxs.Map(res.StackTrace, func(frame apitype.StackFrameV1) resource.StackFrame {
		return resource.StackFrame{SourcePosition: frame.SourcePosition}
	}))

	return resource.NewState{
			Type:                    res.Type,
			URN:                     res.URN,
			Custom:                  res.Custom,
			Delete:                  res.Delete,
			ID:                      res.ID,
			Inputs:                  inputs,
			Outputs:                 outputs,
			Parent:                  res.Parent,
			Protect:                 res.Protect,
			Taint:                   res.Taint,
			External:                res.External,
			Dependencies:            res.Dependencies,
			InitErrors:              res.InitErrors,
			Provider:                res.Provider,
			PropertyDependencies:    res.PropertyDependencies,
			PendingReplacement:      res.PendingReplacement,
			AdditionalSecretOutputs: res.AdditionalSecretOutputs,
			Aliases:                 res.Aliases,
			CustomTimeouts:          res.CustomTimeouts,
			ImportID:                res.ImportID,
			RetainOnDelete:          res.RetainOnDelete,
			DeletedWith:             res.DeletedWith,
			Created:                 res.Created,
			Modified:                res.Modified,
			SourcePosition:          res.SourcePosition,
			StackTrace:              stackTrace,
			IgnoreChanges:           res.IgnoreChanges,
			ReplaceOnChanges:        res.ReplaceOnChanges,
			RefreshBeforeUpdate:     res.RefreshBeforeUpdate,
			ViewOf:                  res.ViewOf,
			ResourceHooks:           res.ResourceHooks,
		}.Make(),
		nil
}

// DeserializeOperation hydrates a pending resource/operation pair.
func DeserializeOperation(op apitype.OperationV2, dec config.Decrypter,
) (resource.Operation, error) {
	res, err := DeserializeResource(op.Resource, dec)
	if err != nil {
		return resource.Operation{}, err
	}
	return resource.NewOperation(res, resource.OperationType(op.Type)), nil
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]interface{}, dec config.Decrypter,
) (resource.PropertyMap, error) {
	result := make(resource.PropertyMap)
	for k, prop := range props {
		desprop, err := DeserializePropertyValue(prop, dec)
		if err != nil {
			return nil, err
		}
		result[resource.PropertyKey(k)] = desprop
	}
	return result, nil
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v interface{}, dec config.Decrypter,
) (resource.PropertyValue, error) {
	ctx := context.TODO()
	if v != nil {
		switch w := v.(type) {
		case bool:
			return resource.NewProperty(w), nil
		case float64:
			return resource.NewProperty(w), nil
		case string:
			if w == computedValuePlaceholder {
				return resource.MakeComputed(resource.NewProperty("")), nil
			}
			return resource.NewProperty(w), nil
		case []interface{}:
			arr := make([]resource.PropertyValue, len(w))
			for i, elem := range w {
				ev, err := DeserializePropertyValue(elem, dec)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				arr[i] = ev
			}
			return resource.NewProperty(arr), nil
		case map[string]interface{}:
			obj, err := DeserializeProperties(w, dec)
			if err != nil {
				return resource.PropertyValue{}, err
			}

			// This could be an asset or archive; if so, recover its type.
			objmap := obj.Mappable()
			if sig, hasSig := objmap[resource.SigKey]; hasSig {
				switch sig {
				case asset.AssetSig:
					asset, isasset, err := asset.Deserialize(objmap)
					if err != nil {
						return resource.PropertyValue{}, err
					}
					contract.Assertf(isasset, "resource with asset signature is not an asset")
					return resource.NewProperty(asset), nil
				case archive.ArchiveSig:
					archive, isarchive, err := archive.Deserialize(objmap)
					if err != nil {
						return resource.PropertyValue{}, err
					}
					contract.Assertf(isarchive, "resource with archive signature is not an archive")
					return resource.NewProperty(archive), nil
				case resource.SecretSig:
					prop := resource.MakeSecret(resource.NewNullProperty())
					secret := prop.SecretValue()
					ciphertext, cipherOk := objmap["ciphertext"].(string)
					plaintext, plainOk := objmap["plaintext"].(string)
					if (!cipherOk && !plainOk) || (plainOk && cipherOk) {
						return resource.PropertyValue{}, errors.New(
							"malformed secret value: exactly one of `ciphertext` or `plaintext` must be supplied")
					}

					if plainOk {
						propertyValue, err := secretPropertyValueFromPlaintext(plaintext)
						if err != nil {
							return resource.PropertyValue{}, err
						}
						secret.Element = propertyValue
					} else {
						// If the decrypter supports batching, use the Enqueue method to asynchronously decrypt the secret value.
						if batchDecrypter, ok := dec.(BatchDecrypter); ok {
							err := batchDecrypter.Enqueue(ctx, ciphertext, secret)
							if err != nil {
								return resource.PropertyValue{}, fmt.Errorf("enqueuing secret value for decryption: %w", err)
							}
						} else {
							unencryptedText, err := dec.DecryptValue(ctx, ciphertext)
							if err != nil {
								return resource.PropertyValue{}, fmt.Errorf("decrypting secret value: %w", err)
							}
							ev, err := secretPropertyValueFromPlaintext(unencryptedText)
							if err != nil {
								return resource.PropertyValue{}, err
							}
							secret.Element = ev
						}
					}

					return prop, nil
				case resource.ResourceReferenceSig:
					var packageVersion string
					if packageVersionV, ok := objmap["packageVersion"]; ok {
						packageVersion, ok = packageVersionV.(string)
						if !ok {
							return resource.PropertyValue{},
								errors.New("malformed resource reference: packageVersion must be a string")
						}
					}

					urnStr, ok := objmap["urn"].(string)
					if !ok {
						return resource.PropertyValue{}, errors.New("malformed resource reference: missing urn")
					}
					urn := resource.URN(urnStr)

					// deserializeID handles two cases, one of which arose from a bug in a refactoring of resource.ResourceReference.
					// This bug caused the raw ID PropertyValue to be serialized as a map[string]interface{}. In the normal case, the
					// ID is serialized as a string.
					deserializeID := func() (string, bool, error) {
						idV, ok := objmap["id"]
						if !ok {
							return "", false, nil
						}

						switch idV := idV.(type) {
						case string:
							return idV, true, nil
						case map[string]interface{}:
							switch v := idV["V"].(type) {
							case nil:
								// This happens for component resource references, which do not have an associated ID.
								return "", false, nil
							case string:
								// This happens for custom resource references, which do have an associated ID.
								return v, true, nil
							case map[string]interface{}:
								// This happens for custom resource references with an unknown ID. In this case, the ID should be
								// deserialized as the empty string.
								return "", true, nil
							}
						}
						return "", false, errors.New("malformed resource reference: id must be a string")
					}

					id, hasID, err := deserializeID()
					if err != nil {
						return resource.PropertyValue{}, err
					}
					if hasID {
						return resource.MakeCustomResourceReference(urn, resource.ID(id), packageVersion), nil
					}
					return resource.MakeComponentResourceReference(urn, packageVersion), nil
				default:
					return resource.PropertyValue{}, fmt.Errorf("unrecognized signature '%v' in property map", sig)
				}
			}

			// Otherwise, it's just a weakly typed object map.
			return resource.NewProperty(obj), nil
		default:
			contract.Failf("Unrecognized property type %T: %v", v, reflect.ValueOf(v))
		}
	}

	return resource.NewNullProperty(), nil
}

func secretPropertyValueFromPlaintext(plaintext string) (resource.PropertyValue, error) {
	var elem any
	if err := json.Unmarshal([]byte(plaintext), &elem); err != nil {
		return resource.PropertyValue{}, err
	}
	return DeserializePropertyValue(elem, config.NopDecrypter)
}

// FormatDeploymentDeserializationError formats deployment-related errors into user-friendly messages.
// It handles version compatibility errors and unsupported feature errors.
func FormatDeploymentDeserializationError(err error, stackName string) error {
	var unsupportedErr *ErrDeploymentUnsupportedFeatures

	switch {
	case errors.As(err, &unsupportedErr):
		return fmt.Errorf(
			"the stack '%s' uses features that are not supported by this version of the Pulumi CLI: %s. "+
				"Please update your version of the Pulumi CLI",
			stackName, strings.Join(unsupportedErr.Features, ", "))
	case errors.Is(err, ErrDeploymentSchemaVersionTooOld):
		return fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
			stackName)
	case errors.Is(err, ErrDeploymentSchemaVersionTooNew):
		return fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
			"Please update your version of the Pulumi CLI", stackName)
	}
	return fmt.Errorf("could not deserialize deployment: %w", err)
}
