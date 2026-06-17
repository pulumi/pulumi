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

// Package oss provides a first-class Alibaba Cloud OSS backend for Pulumi state
// storage.
//
// OSS exposes an S3-compatible API, so this package does not implement a new
// storage driver: it registers an oss:// scheme on the default blob.URLMux that
// bridges to gocloud's s3blob driver pointed at the OSS S3-compatible endpoint.
// Registration happens automatically during initialization, so importing the
// package (the DIY backend does so) is enough to enable oss:// URLs.
//
// The oss:// URL takes the bucket as its host and a region (required) as a query
// parameter. The S3-compatible endpoint https://s3.oss-<region>.aliyuncs.com is
// derived from the region unless an explicit endpoint is supplied:
//
//	oss://my-pulumi-state-bucket?region=cn-hangzhou
//	oss://my-pulumi-state-bucket?region=cn-hangzhou&endpoint=https://s3.oss-cn-hangzhou-internal.aliyuncs.com
//
// Credentials are read from the standard Alibaba Cloud environment variables
// ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET, falling back
// to the AWS SDK default credential chain when those are unset.
//
// Two OSS S3-compatibility quirks are handled for the user: virtual-hosted-style
// addressing (the s3blob default — OSS rejects path-style) and the request
// checksum calculation, which is forced to "when required" so OSS does not reject
// uploads with the aws-chunked content-encoding that recent AWS SDK versions add
// by default.
package oss
