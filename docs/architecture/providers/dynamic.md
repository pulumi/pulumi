(dynamic-providers)=
# Dynamic providers

[*Dynamic
providers*](https://www.pulumi.com/docs/concepts/resources/dynamic-providers/)
are a Pulumi feature that allows the core logic of a provider to be defined and
managed within the context of a Pulumi program. This is in contrast to a normal
("real", sometimes "side-by-side") provider, whose logic is encapsulated as a
separate [plugin](plugins) for use in any program. Dynamic providers are
presently only supported in NodeJS/TypeScript and Python. They work as follows:

* The SDK defines two types:
  * That of *dynamic providers* -- objects with methods for the lifecycle
    methods that a gRPC provider would normally offer (CRUD, diff, etc.).
  * That of *dynamic resources* -- those that are managed by a dynamic provider.
    This type specialises (e.g. by subclassing in NodeJS and Python) the SDK's
    core resource type so that all dynamic resources *have the same Pulumi
    package* -- `pulumi-nodejs` for NodeJS and `pulumi-python` for Python.

  These are located in <gh-file:pulumi#sdk/nodejs/dynamic/index.ts> in
  NodeJS/TypeScript and
  <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/dynamic.py> in Python.
* The SDK also defines a "real" provider that implements the gRPC interface and
  manages the lifecycle of dynamic resources. This provider is named according
  to the single package name used for all dynamic resources. See
  <gh-file:pulumi#sdk/nodejs/cmd/dynamic-provider/index.ts> for NodeJS and
  <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/__main__.py> for Python.

* A user extends the types defined by the SDK in order to implement one or more
  dynamic providers and resources that belong to those providers. They use these
  resources in their program like any other.
* When a dynamic resource class is instantiated, it *captures the provider
  instance that manages it* and *serializes this provider instance* as part of
  the resource's properties.
  * In NodeJS, serialization is performed by capturing and mangling the source
    code of the provider and any dependencies by (ab)using v8 primitives -- see
    <gh-file:pulumi#sdk/nodejs/runtime/closure> for the gory details.
  * In Python, serialization is performed by pickling the dynamic provider
    instance -- see <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/dynamic.py>'s
    use of `dill` for more on this.
* The serialized provider state is then stored as a property on the dynamic
  resource. It is consequently sent to the engine as part of lifecycle calls
  (check, diff, create, etc.) like any other property.
* When the engine receives requests pertaining to dynamic resources, the fixed
  package (`pulumi-nodejs` or `pulumi-python`) will cause it to make provider
  calls against the "real" provider defined in the SDK.
* The provider proxies these calls to the code the user wrote by deserializing
  and hydrating the provider instance from the resource's properties and
  invoking the appropriate code.

These implementation choices impose a number of limitations:

* Serialized/pickled code is brittle and simply doesn't work in all cases. Some
  features are supported and some aren't, depending on the language and
  surrounding context. Dependency management (both within the user's program and
  as it relates to third-party packages such as those from NPM or PyPi) is
  challenging.
* Even when code works once, or in one context, it might not work later on. If
  e.g. absolute paths specific to one machine form part of the provider's code
  (or the code of its dependencies), the fact that these are serialized into the
  Pulumi state means that on later hydration, a program that worked before might
  not work again.
* Related to the problem of state serialization is the fact that dynamic
  provider state is only updated *when the program runs*. It is therefore not
  possible in general to e.g. change the code of a dynamic provider and expect
  an operation like `destroy` (which does not run the program) to pick up the
  changes.
