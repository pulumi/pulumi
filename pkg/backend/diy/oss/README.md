(oss-backend)=
# Alibaba Cloud OSS Backend for Pulumi

The `pkg/backend/diy/oss` package provides a first-class Alibaba Cloud OSS backend
for Pulumi state storage. OSS exposes an S3-compatible API, so this package does
not implement a new storage driver — it registers an `oss://` scheme that bridges
to gocloud's `s3blob` driver pointed at the OSS S3-compatible endpoint, wiring up
the endpoint, addressing style, and request-checksum behavior that OSS requires.

## Connection String Format

```text
oss://bucket-name?region=<region>
```

The bucket is the URL host and the region is required. A trailing path is treated
as a key prefix within the bucket (matching the other DIY object-storage backends):

```text
oss://bucket-name/path/prefix?region=<region>
```

## Configuration Options

The following query parameters are supported:

- `region` (required): The OSS region, for example `cn-hangzhou` or `us-west-1`.
  Both bare (`cn-hangzhou`) and prefixed (`oss-cn-hangzhou`) forms are accepted.
  The endpoint `https://s3.oss-<region>.aliyuncs.com` is derived from it.
- `endpoint` (optional): An explicit S3-compatible endpoint that overrides the one
  derived from `region`. Use this for internal/VPC endpoints, for example
  `https://s3.oss-cn-hangzhou-internal.aliyuncs.com`.

## Usage

```bash
export ALIBABA_CLOUD_ACCESS_KEY_ID=<your-access-key-id>
export ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your-access-key-secret>

pulumi login "oss://my-pulumi-state-bucket?region=cn-hangzhou"
```

## Credentials

Credentials are resolved from the standard Alibaba Cloud environment variables:

```bash
export ALIBABA_CLOUD_ACCESS_KEY_ID=<your-access-key-id>
export ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your-access-key-secret>
```

These are OSS AccessKeys from the Alibaba Cloud console (or a RAM user) — they are
a separate system from AWS IAM credentials. When the Alibaba Cloud variables are
unset, the AWS SDK default credential chain (`AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, shared config) is used instead.

## S3 Compatibility Notes

- **Virtual-hosted-style addressing.** OSS rejects path-style requests, so this
  backend uses the `s3blob` default (virtual-hosted style). Do not force path style.
- **SigV4.** Requests are signed with SigV4. The region is required for signing
  even though OSS ignores its value.
- **Request checksums.** The request checksum calculation is forced to
  "when required" so OSS does not reject uploads with the `aws-chunked`
  content-encoding that recent AWS SDK versions add by default (otherwise OSS
  returns `403 SignatureDoesNotMatch`).

## Generic S3-Compatible Stores

Any S3-compatible store can also be used directly through the `s3://` scheme with
an `endpoint` query parameter (MinIO, Ceph, Cloudflare R2, DigitalOcean Spaces,
and others). When a custom `endpoint` is supplied, the DIY backend defaults the
request checksum calculation to "when required" for the same compatibility reason:

```bash
pulumi login "s3://my-bucket?endpoint=https://s3.oss-cn-hangzhou.aliyuncs.com&region=cn-hangzhou"
```

The branded `oss://` scheme is a convenience wrapper over this path for OSS.

## Limitations

- OSS returns ETags in uppercase and uses a different multipart ETag algorithm
  than AWS S3; tools that compare ETags for integrity may observe differences.
- OSS supports a subset of S3 ACLs (`private`, `public-read`, `public-read-write`).
