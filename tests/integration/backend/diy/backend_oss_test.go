// Copyright 2026, Pulumi Corporation.
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

package diy

import (
	"fmt"
	"os"
	"testing"
)

// ossTestParams reads the Alibaba Cloud OSS test configuration from the
// environment, skipping the test when it is not fully configured. OSS is not
// available in CI by default, so these tests run only when credentials and a
// test bucket are provided:
//
//	ALIBABA_CLOUD_ACCESS_KEY_ID, ALIBABA_CLOUD_ACCESS_KEY_SECRET,
//	PULUMI_TEST_OSS_BUCKET, PULUMI_TEST_OSS_REGION
func ossTestParams(t *testing.T) (bucket, region string) {
	t.Helper()

	if os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID") == "" ||
		os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET") == "" {
		t.Skip("Skipping, ALIBABA_CLOUD_ACCESS_KEY_ID/ALIBABA_CLOUD_ACCESS_KEY_SECRET not set")
	}

	bucket = os.Getenv("PULUMI_TEST_OSS_BUCKET")
	region = os.Getenv("PULUMI_TEST_OSS_REGION")
	if bucket == "" || region == "" {
		t.Skip("Skipping, PULUMI_TEST_OSS_BUCKET/PULUMI_TEST_OSS_REGION not set")
	}

	return bucket, region
}

//nolint:paralleltest // this test sets the global login state
func TestOSSLogin(t *testing.T) {
	t.Chdir("project")

	bucket, region := ossTestParams(t)

	cloudURL := fmt.Sprintf("oss://%s?region=%s", bucket, region)
	loginAndCreateStack(t, cloudURL)
}

//nolint:paralleltest // this test sets the global login state
func TestOSSLoginViaS3Endpoint(t *testing.T) {
	t.Chdir("project")

	bucket, region := ossTestParams(t)

	// Exercise the generic S3-compatible path: an s3:// URL with a custom OSS
	// endpoint. The DIY backend defaults request_checksum_calculation to
	// "when required" for custom endpoints, which is what keeps OSS uploads from
	// failing with 403 SignatureDoesNotMatch.
	t.Setenv("AWS_ACCESS_KEY_ID", os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"))
	t.Setenv("AWS_SECRET_ACCESS_KEY", os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"))

	cloudURL := fmt.Sprintf(
		"s3://%s?endpoint=https://s3.oss-%s.aliyuncs.com&region=%s",
		bucket, region, region)
	loginAndCreateStack(t, cloudURL)
}
