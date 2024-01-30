package diy

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
)

func getAwsCaller(t *testing.T) (context.Context, aws.Config) {
	t.Helper()
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Logf("Skipping, could not load aws config: %s", err)
		t.SkipNow()
	}

	stsClient := sts.NewFromConfig(cfg)
	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Logf("Skipping, couldn't use aws credentials to query identity: %s", err)
		t.SkipNow()
	}

	return ctx, cfg
}

//nolint:paralleltest // this test sets the global login state
func TestAwsLogin(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})

	t.Setenv("AWS_REGION", "us-west-2")

	ctx, cfg := getAwsCaller(t)

	creds, err := cfg.Credentials.Retrieve(ctx)
	require.NoError(t, err)

	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	cloudURL := "s3://pulumitesting"
	loginAndCreateStack(t, cloudURL)
}
