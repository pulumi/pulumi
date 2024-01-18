package authhelpers

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

func S3BuildSessionOptions(ctx context.Context, backend *workspace.ProjectBackend) session.Options {
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

	return session.Options{
		// It's okay for the value to be "" here.  That will induce the fallback
		// behavior when used in session.NewSessionWithOptions().  Which is checking
		// for a non-zero-length string
		Profile: definitiveProfileName,
	}

}

func S3CredentialsMux(ctx context.Context, backend *workspace.ProjectBackend) (*blob.URLMux, error) {
	// Returns a blobmux only registered to handle s3, and do so in our specially defined way
	sess, err := session.NewSessionWithOptions(S3BuildSessionOptions(ctx, backend))
	if err != nil {
		return nil, err
	}

	blobmux := &blob.URLMux{}
	blobmux.RegisterBucket(s3blob.Scheme, &s3blob.URLOpener{
		ConfigProvider: sess,
	})

	return blobmux, nil
}
