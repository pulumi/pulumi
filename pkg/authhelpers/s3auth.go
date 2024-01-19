package authhelpers

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

func S3BuildSessionOptions(ctx context.Context, backend *workspace.ProjectBackend) (*session.Options, error) {
	// Select the session options based on the backend setting, superceded by the PULUMI_ env var if set.
	// If neither set, fall back to default session builder (Which interprets AWS_ environment vars first)

	definitiveProfileName := ""
	if backend != nil && backend.AwsProfileName != "" {
		definitiveProfileName = backend.AwsProfileName
	}

	profileNameEnv := os.Getenv("PULUMI_BACKEND_AWS_PROFILE_NAME")
	if profileNameEnv != "" {
		definitiveProfileName = profileNameEnv
	}

	opts := session.Options{}
	if definitiveProfileName != "" {
		// Get config for profile
		profileConfig, err := config.LoadSharedConfigProfile(ctx, definitiveProfileName)
		if err != nil {
			return nil, err
		}
		opts.Profile = definitiveProfileName
		opts.Config = aws.Config{
			Region: &profileConfig.Region,
		}
	}
	return &opts, nil
}

func S3CredentialsMux(ctx context.Context, backend *workspace.ProjectBackend) (*blob.URLMux, error) {
	// Returns a blobmux only registered to handle s3, and do so in our specially defined way
	opts, err := S3BuildSessionOptions(ctx, backend)
	if err != nil {
		return nil, err
	}

	sess, err := session.NewSessionWithOptions(*opts)
	if err != nil {
		return nil, err
	}

	blobmux := &blob.URLMux{}
	blobmux.RegisterBucket(s3blob.Scheme, &s3blob.URLOpener{
		ConfigProvider: sess,
	})

	return blobmux, nil
}
