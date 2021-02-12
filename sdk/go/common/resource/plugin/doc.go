package plugin

/*
Package plugin defines the plugin interfaces used by the Pulumi engine and provides basic plugin management for
out-of-process plugin implementations that communicate over gRPC.

Providers

The Provider type defines the interface that must be implemented by a Pulumi resource provider. Resource providers
use the resource.PropertyValue types as their basic format for exchanging data with the engine. These types represent
a superset of the value representable in JSON; they are documented in their containing package.

Over time, the lifecycle of a resource provider has become more complex, and the Provider interface has accumulated
a number of quirks intended to retain backwards compatibility with older versions of the Pulumi engine. These factors
combined make it difficult to understand what exactly the requirements are for a provider that targets a particular
version of the plugin interface. The lifecycle and interface requirements are described in
provider-implementers-guide.md.

Analyzers

TBD

Language Hosts

TBD
*/
