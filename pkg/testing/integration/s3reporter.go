package integration

import integration "github.com/pulumi/pulumi/sdk/v3/pkg/testing/integration"

// S3Reporter is a TestStatsReporter that publises test data to S3
type S3Reporter = integration.S3Reporter

// NewS3Reporter creates a new S3Reporter that puts test results in the given bucket using the keyPrefix.
func NewS3Reporter(region string, bucket string, keyPrefix string) *S3Reporter {
	return integration.NewS3Reporter(region, bucket, keyPrefix)
}

