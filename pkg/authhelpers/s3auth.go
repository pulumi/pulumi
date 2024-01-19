package authhelpers

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

const (
	logPrefix                      string = "S3 Backend: "
	profileNameEnvironmentVariable string = "PULUMI_BACKEND_AWS_PROFILE_NAME"
)

var (
	regionFinder         = s3FindRegionFromProfile
	sessionOptionBuilder = s3BuildSessionOptions
)

// s3FindRegionFromProfile discovers the region from shared config for a given AWS
// profile.  Uses config locations as determined by the aws SDK
func s3FindRegionFromProfile(ctx context.Context, profile string) (string, error) {
	profileConfig, err := config.LoadSharedConfigProfile(ctx, profile)
	if err != nil {
		return "", err
	}
	return profileConfig.Region, nil
}

// s3BuildSessionOptions Constructs session options from passed ProjectBackend configuration
// uses the AwsProfileName param in backend config, or via PULUMI_BACKEND_AWS_PROFILE_NAME,
// attempts to load shared configuration, and passes back configured options with the profile name
// and configured region for this profile
func s3BuildSessionOptions(ctx context.Context, backend *workspace.ProjectBackend) (*session.Options, error) {
	log := logging.V(5)

	definitiveProfileName := ""
	if backend != nil && backend.AwsProfileName != "" {
		definitiveProfileName = backend.AwsProfileName
		log.Infof(
			"%sUsing aws profile \"%s\" from project configuration",
			logPrefix,
			definitiveProfileName,
		)
	}

	profileNameEnv := os.Getenv(profileNameEnvironmentVariable)
	if profileNameEnv != "" {
		definitiveProfileName = profileNameEnv
		log.Infof(
			"%sOverriding aws profile \"%s\" from environment configuration (%s)",
			logPrefix,
			definitiveProfileName,
			profileNameEnvironmentVariable,
		)
	}

	opts := session.Options{}
	if definitiveProfileName != "" {
		// Get configured region for profile
		region, err := regionFinder(ctx, definitiveProfileName)
		if err != nil {
			return nil, err
		}
		opts.Profile = definitiveProfileName
		opts.Config = aws.Config{
			Region: &region,
		}
		log.Infof(
			"%sSelected profile \"%s\" and region \"%s\" from profile config for backend auth",
			logPrefix,
			definitiveProfileName,
			region,
		)
	} else {
		log.Infof(
			"%sNo profile configuration was provided for S3 auth.  Defaulting to environment then default aws profile",
			logPrefix,
		)
	}
	return &opts, nil
}

// S3CredentialsMux returns a blob.URLMux with a registered Opener for s3:// schemas
// The registered opener is constructed based on backend settings, and if configured, an
// session built off of a specfically configured AWS profile.
func S3CredentialsMux(ctx context.Context, backend *workspace.ProjectBackend) (*blob.URLMux, error) {
	opts, err := sessionOptionBuilder(ctx, backend)
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
