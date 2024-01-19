package authhelpers

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func mockRegionFinder(_ context.Context, _ string) (string, error) {
	return "us-west-2", nil
}

func harnessTempAWSConfig(t *testing.T) {
	// Mocks out any helper functions which have a hard time being run in the test environment

	// RegionFinder is based on awsv2/config/LoadSharedConfigProfile.  This is really home-directory
	// dependant.  For now, I'm not offerring the ability to configure static config and credential
	// file locations, so we need to mock this out for testing.
	regionFinder = mockRegionFinder
}

//nolint:paralleltest
func TestS3BuildSessionOptions_Unconfigured(t *testing.T) {
	harnessTempAWSConfig(t)
	ctx := context.Background()
	backendConfig := workspace.ProjectBackend{}

	options, err := s3BuildSessionOptions(ctx, &backendConfig)

	assert.NoError(t, err)
	assert.NotNil(t, options)

	assert.Equal(t, options.Profile, "")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_Backend(t *testing.T) {
	harnessTempAWSConfig(t)
	hd, _ := os.UserHomeDir()
	fmt.Println(hd)
	ctx := context.Background()
	backendConfig := workspace.ProjectBackend{
		AwsProfileName: "distinct",
	}

	options, err := s3BuildSessionOptions(ctx, &backendConfig)

	assert.NoError(t, err)
	assert.NotNil(t, options)

	assert.Equal(t, options.Profile, "distinct")
	assert.Equal(t, *options.Config.Region, "us-west-2")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_EnvVar(t *testing.T) {
	harnessTempAWSConfig(t)
	ctx := context.Background()
	t.Setenv("PULUMI_BACKEND_AWS_PROFILE_NAME", "configuredwithenv")
	backendConfig := workspace.ProjectBackend{}

	options, err := s3BuildSessionOptions(ctx, &backendConfig)

	assert.NoError(t, err)
	assert.NotNil(t, options)

	assert.Equal(t, options.Profile, "configuredwithenv")
	assert.Equal(t, *options.Config.Region, "us-west-2")
}

//nolint:paralleltest
func TestS3BuildSessionOptions_Superceded(t *testing.T) {
	harnessTempAWSConfig(t)
	ctx := context.Background()
	t.Setenv("PULUMI_BACKEND_AWS_PROFILE_NAME", "configuredwithenv")
	backendConfig := workspace.ProjectBackend{
		AwsProfileName: "distinct",
	}

	options, err := s3BuildSessionOptions(ctx, &backendConfig)

	assert.NoError(t, err)
	assert.NotNil(t, options)

	assert.Equal(t, options.Profile, "configuredwithenv")
	assert.Equal(t, *options.Config.Region, "us-west-2")
}
