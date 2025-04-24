(default-providers)=
# Default providers

A *default provider* for a package and version is the provider instance that
Pulumi will use to manage resources that do not have a provider explicitly
specified (either directly as a resource option or indirectly via a parent, for
instance). Consider for example the following TypeScript program that creates
an S3 bucket in AWS:

```typescript
import * as aws from "@pulumi/aws"

new aws.s3.Bucket("my-bucket")
```

The `Bucket` constructor will yield a [](pulumirpc.RegisterResourceRequest) such
as the following:

```
RegisterResourceRequest{
  type: "aws:s3/bucket:Bucket",
  name: "my-bucket",
  parent: "urn:pulumi:dev::project::pulumi:pulumi:Stack::project",
  custom: true,
  object: {},
  version: "4.16.0",
}
```

The absence of a `provider` field in this request will cause the engine to use a
default provider for the `aws` package at version 4.16.0. The engine's
[](pulumirpc.ResourceMonitor) implementation ensures that only a single default
provider instance exists for each package version, and only creates default
provider instances on demand (that is, when a resource that requires one is
registered). Default provider instances are created by synthesizing appropriate
`RegisterResourceEvent`s with inputs sourced from the stack's configuration
values for the relevant provider package. In the example above, the default AWS
provider would be configured using any stack configuration values whose keys
begin with `aws:` (e.g. `aws:region`).

Changing the above example to use an explicit provider will prevent a default
provider from being used:

```typescript
import * as aws from "@pulumi/aws"

const usWest2 = new aws.Provider("us-west-2", { region: "us-west-2" })

new aws.s3.Bucket("my-bucket", {}, { provider: usWest2 })
```

This will yield a `RegisterResourceRequest` whose `provider` field references
the explicitly constructed entity:

```
RegisterResourceRequest{
  type: "aws:s3/bucket:Bucket",
  name: "my-bucket",
  parent: "urn:pulumi:dev::project::pulumi:pulumi:Stack::project",
  custom: true,
  object: {},
  provider: "urn:pulumi:dev::project::pulumi:providers:aws::us-west-2::308b79ee-8249-40fb-a203-de190cb8faa8",
  version: "4.16.0",
}
```

Note that the explicit provider *itself* is registered as a resource, and its
constructor will emit its own `RegisterResourceRequest` with the appropriate
name, type, parent, and so on.
