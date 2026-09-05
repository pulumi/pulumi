# Resources

Pulumi has four different types of resources.  Provider Resources,
Custom Resources, Component Resources and View Resources.  We'll
describe them below.

## Provider Resources

Provider resources are virtual (they only exist in the Pulumi state,
not on any backend) resources, that represent a provider and its
configuration. They are stored in state, and referenced by custom
resources, so the engine knows which provider, with which settings was
used to set up the resource, and so the same settings can be used for
further update or destroy operation for a resource.

## Custom Resources

Custom resources are resources that are created in a provider. Custom
resources often exist in a cloud provider, but this is not necessary
for all resources (e.g. see the pulumi-random provider, that creates
resources and stores all of the state in outputs, without creating any
resources in the cloud backend)

## Component Resources

Component resources are usually used as a container to group other
resources together, and to create reusable components that consist of
other resources. Similar to Provider resources, these resources are
also virtual resources, and are only represented within Pulumi state.

The model for these depends on the implementation language of the
component resource, and most commonly they are implemented as classes
where the programming language supports that.

(view-resources)=
## View Resources

View resources represent a view of a resource that's created
externally to Pulumi. These resources are created by a provider, as
part of a custom resource, and represent a "view" of resources managed
externally to Pulumi. These can e.g. be resources set up through
OpenTofu, but as part of a Pulumi Program.  Pulumi doesn't manage the
lifecycle of this type of resource itself, but rather defers to the
provider to do so.

## Resource Existence Checks

Pulumi provides an `exists` method on custom resource types that
checks whether a resource with a given ID currently exists in the
provider. Unlike `get` (which reads and imports the resource into
state), `exists` performs a read-only check and returns
`Output<bool>` without registering the resource in the deployment
state.

The `exists` method is implemented using the provider's `Read` RPC.
If `Read` returns output properties, the resource exists; if it
returns nil outputs, the resource does not exist.

### SDK Usage

Each language SDK exposes `exists` as a static method on resource
classes:

- **Go**: `ResourceExists(ctx, name, id, state, opts...)`
- **Node.js**: `Resource.exists(name, id, state?, opts?)`
- **Python**: `Resource.exists(resource_name, id, opts=None, **state_kwargs)`

### Engine Implementation

The `ExistsResource` RPC is handled directly in the resource
monitor without going through the step pipeline. It resolves the
provider (including default provider registration if needed),
calls `provider.Read()`, and returns a boolean result. No
resources are registered in the deployment snapshot as a result of
this call.
