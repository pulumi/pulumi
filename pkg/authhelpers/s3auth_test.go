package authhelpers

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest
func TestS3BuildSessionOptions_Unconfigured(t *testing.T) {
	ctx := context.Background()
	backendConfig := workspace.ProjectBackend{}

	options := S3BuildSessionOptions(ctx, &backendConfig)

	assert.Equal(t, options.Profile, "")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_Backend(t *testing.T) {
	ctx := context.Background()
	backendConfig := workspace.ProjectBackend{
		AwsProfileName: "distinct",
	}

	options := S3BuildSessionOptions(ctx, &backendConfig)

	assert.Equal(t, options.Profile, "distinct")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_EnvVar(t *testing.T) {
	ctx := context.Background()
	t.Setenv("PULUMI_BACKEND_AWS_PROFILE_NAME", "configuredwithenv")
	backendConfig := workspace.ProjectBackend{}

	options := S3BuildSessionOptions(ctx, &backendConfig)

	assert.Equal(t, options.Profile, "configuredwithenv")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_Superceded(t *testing.T) {
	ctx := context.Background()
	t.Setenv("PULUMI_BACKEND_AWS_PROFILE_NAME", "configuredwithenv")
	backendConfig := workspace.ProjectBackend{
		AwsProfileName: "distinct",
	}

	options := S3BuildSessionOptions(ctx, &backendConfig)

	assert.Equal(t, options.Profile, "configuredwithenv")
}
