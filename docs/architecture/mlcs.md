(mlcs)=
# Multi-language components (MLCs)

Authors of Pulumi programs can use [component
resources](https://www.pulumi.com/docs/concepts/resources/components/) to
logically group related resources together. For instance, a TypeScript program
might specify a component that combines AWS and PostgreSQL providers to abstract
the management of an RDS database and logical databases within it:

```typescript
import * as aws from "@pulumi/aws"
import * as postgresql from "@pulumi/postgresql"

class Database extends pulumi.ComponentResource {
  constructor(name: string, args: DatabaseArgs, opts?: pulumi.ComponentResourceOptions) {
    super("my:database:Database", name, args, opts)

    const rds = new aws.rds.Instance("my-rds", { ... }, { parent: this })
    const pg = new postgresql.Database("my-db", { ... }, { parent: this })

    ...
  }
}
```

This component can then be used just like any other Pulumi resource:

```typescript
const db = new Database("my-db", { ... })
```

...if the program is written in the same language as the component (in this
case, TypeScript). In some cases however it would be great if components could
be reused in multiple languages, since components provide a natural means to
abstract and reuse infrastructure.

Enter *multi-language components* (MLCs). MLCs are components which can be
written in one language and used in another (or rather, any other). Under the
hood, MLCs are implemented as just another [](pulumirpc.ResourceProvider)
method: [](pulumirpc.ResourceProvider.Construct). The engine automatically calls
`Construct` when it sees a request to create an MLC.[^engine-construct] Indeed,
since providers and gRPC calls are the key to making custom resources consumable
in any language, exposing components through the same interface is a natural
extension of the Pulumi model.

[^engine-construct]:
    See [resource registration](resource-monitor) for more information.

Just as the body of a component resource is largely concerned with instantiating
other resources, so is the implementation of `Construct` for an MLC. Whereas a
custom resource's [](pulumirpc.ResourceProvider.Create) method can be expected
to make a "raw" call to some underlying cloud provider API (for instance),
[](pulumirpc.ResourceProvider.Construct) is generally only concerned with
registering child resources and their desired state. For this reason,
[](pulumirpc.ConstructRequest) includes a `monitorEndpoint` so that the MLC can
itself make [](pulumirpc.ResourceMonitor.RegisterResource) calls *back* to the [deployment's
resource monitor](resource-monitor) to register these child resources.
