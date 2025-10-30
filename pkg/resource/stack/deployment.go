package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/resource/stack"

// ErrDeploymentUnsupportedFeatures is returned from `DeserializeDeployment` if the
// untyped deployment being deserialized uses one or more features that are not supported.
type ErrDeploymentUnsupportedFeatures = stack.ErrDeploymentUnsupportedFeatures

// SerializeOptions controls how a deployment is serialized to JSON.
type SerializeOptions = stack.SerializeOptions

const DeploymentSchemaVersionOldestSupported = stack.DeploymentSchemaVersionOldestSupported

const DeploymentSchemaVersionLatest = stack.DeploymentSchemaVersionLatest

var ErrDeploymentSchemaVersionTooOld = stack.ErrDeploymentSchemaVersionTooOld

var ErrDeploymentSchemaVersionTooNew = stack.ErrDeploymentSchemaVersionTooNew

// ApplyFeatures applies the features used by a resource to the feature map.
func ApplyFeatures(res apitype.ResourceV3, features map[string]bool) {
	stack.ApplyFeatures(res, features)
}

// ValidateUntypedDeployment validates a deployment against the Deployment JSON schema.
func ValidateUntypedDeployment(deployment *apitype.UntypedDeployment) error {
	return stack.ValidateUntypedDeployment(deployment)
}

// SerializeDeployment serializes an entire snapshot as a deploy record.
func SerializeDeployment(ctx context.Context, snap *deploy.Snapshot, showSecrets bool) (*apitype.DeploymentV3, error) {
	return stack.SerializeDeployment(ctx, snap, showSecrets)
}

// SerializeDeploymentWithMetadata serializes an entire snapshot as a deploy record returning the deployment, version,
// and features used by the deployment.
func SerializeDeploymentWithMetadata(ctx context.Context, snap *deploy.Snapshot, showSecrets bool) (*apitype.DeploymentV3, int, []string, error) {
	return stack.SerializeDeploymentWithMetadata(ctx, snap, showSecrets)
}

// SerializeUntypedDeployment serializes a snapshot into an untyped deployment.
func SerializeUntypedDeployment(ctx context.Context, snap *deploy.Snapshot, opts *SerializeOptions) (*apitype.UntypedDeployment, error) {
	return stack.SerializeUntypedDeployment(ctx, snap, opts)
}

// UnmarshalUntypedDeployment unmarshals a raw untyped deployment into an up to date deployment object.
func UnmarshalUntypedDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) (*apitype.DeploymentV3, error) {
	return stack.UnmarshalUntypedDeployment(ctx, deployment)
}

// DeserializeUntypedDeployment deserializes an untyped deployment and produces a `deploy.Snapshot`
// from it. DeserializeDeployment will return an error if the untyped deployment's version is
// not within the range `DeploymentSchemaVersionCurrent` and `DeploymentSchemaVersionOldestSupported`.
func DeserializeUntypedDeployment(ctx context.Context, deployment *apitype.UntypedDeployment, secretsProv secrets.Provider) (*deploy.Snapshot, error) {
	return stack.DeserializeUntypedDeployment(ctx, deployment, secretsProv)
}

// DeserializeDeploymentV3 deserializes a typed DeploymentV3 into a `deploy.Snapshot`.
func DeserializeDeploymentV3(ctx context.Context, deployment apitype.DeploymentV3, secretsProv secrets.Provider) (*deploy.Snapshot, error) {
	return stack.DeserializeDeploymentV3(ctx, deployment, secretsProv)
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(ctx context.Context, res *resource.State, enc config.Encrypter, showSecrets bool) (apitype.ResourceV3, error) {
	return stack.SerializeResource(ctx, res, enc, showSecrets)
}

// SerializeOperation serializes a resource in a pending state.
func SerializeOperation(ctx context.Context, op resource.Operation, enc config.Encrypter, showSecrets bool) (apitype.OperationV2, error) {
	return stack.SerializeOperation(ctx, op, enc, showSecrets)
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(ctx context.Context, props resource.PropertyMap, enc config.Encrypter, showSecrets bool) (map[string]any, error) {
	return stack.SerializeProperties(ctx, props, enc, showSecrets)
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(ctx context.Context, prop resource.PropertyValue, enc config.Encrypter, showSecrets bool) (any, error) {
	return stack.SerializePropertyValue(ctx, prop, enc, showSecrets)
}

// DeserializeResource turns a serialized resource back into its usual form.
func DeserializeResource(res apitype.ResourceV3, dec config.Decrypter) (*resource.State, error) {
	return stack.DeserializeResource(res, dec)
}

// DeserializeOperation hydrates a pending resource/operation pair.
func DeserializeOperation(op apitype.OperationV2, dec config.Decrypter) (resource.Operation, error) {
	return stack.DeserializeOperation(op, dec)
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]any, dec config.Decrypter) (resource.PropertyMap, error) {
	return stack.DeserializeProperties(props, dec)
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v any, dec config.Decrypter) (resource.PropertyValue, error) {
	return stack.DeserializePropertyValue(v, dec)
}

// FormatDeploymentDeserializationError formats deployment-related errors into user-friendly messages.
// It handles version compatibility errors and unsupported feature errors.
func FormatDeploymentDeserializationError(err error, stackName string) error {
	return stack.FormatDeploymentDeserializationError(err, stackName)
}

