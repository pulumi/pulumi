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

package oss

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

const (
	// OSSScheme is the scheme for Alibaba Cloud OSS backend URLs.
	OSSScheme = "oss"
)

func init() {
	// Register the OSS bucket opener with the default blob.URLMux so the DIY
	// backend recognizes oss:// URLs automatically.
	mux := blob.DefaultURLMux()
	if !mux.ValidBucketScheme(OSSScheme) {
		mux.RegisterBucket(OSSScheme, URLHandler{})
	}
}

// URLHandler opens oss:// URLs by bridging to gocloud's s3blob driver pointed at
// Alibaba Cloud OSS's S3-compatible endpoint.
type URLHandler struct{}

// OpenBucketURL implements blob.BucketURLOpener.
func (URLHandler) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	cfg, err := configFromURL(ctx, u)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	// OSS rejects the aws-chunked/trailing-checksum content-encoding that recent
	// AWS SDK versions add by default, returning 403 SignatureDoesNotMatch. Setting
	// the checksum calculation to "when required" keeps uploads compatible.
	return s3blob.OpenBucket(ctx, client, u.Host, &s3blob.Options{
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	})
}

// configFromURL builds an AWS SDK config targeting Alibaba Cloud OSS from an
// oss:// URL. The bucket is the URL host; the region (required) and an optional
// endpoint override come from the query string.
func configFromURL(ctx context.Context, u *url.URL) (aws.Config, error) {
	if u.Host == "" {
		return aws.Config{}, fmt.Errorf("oss URL %q is missing a bucket name", u.Redacted())
	}

	q := u.Query()
	region := q.Get("region")
	endpoint := q.Get("endpoint")

	if region == "" && endpoint == "" {
		return aws.Config{}, fmt.Errorf(
			"oss URL %q requires a 'region' query parameter (for example oss://my-bucket?region=cn-hangzhou)",
			u.Redacted())
	}
	if endpoint == "" {
		endpoint = ossEndpoint(region)
	}
	if region == "" {
		// SigV4 still needs a region to sign with even though OSS ignores it.
		region = "cn-hangzhou"
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if id, secret, ok := ossCredentials(); ok {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(id, secret, "")))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading OSS credentials: %w", err)
	}

	cfg.BaseEndpoint = aws.String(endpoint)
	cfg.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired

	return cfg, nil
}

// ossEndpoint returns the S3-compatible OSS endpoint for a region, accepting
// both bare regions (cn-hangzhou) and oss-prefixed regions (oss-cn-hangzhou).
func ossEndpoint(region string) string {
	region = strings.TrimPrefix(region, "oss-")
	return fmt.Sprintf("https://s3.oss-%s.aliyuncs.com", region)
}

// ossCredentials resolves OSS access keys from the standard Alibaba Cloud
// environment variables. When unset, it returns ok=false so the AWS SDK default
// credential chain (AWS_* variables, shared config) is used instead.
func ossCredentials() (id, secret string, ok bool) {
	id = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	secret = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	if id != "" && secret != "" {
		return id, secret, true
	}
	return "", "", false
}
