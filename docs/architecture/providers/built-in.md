(built-in-provider)=
# The built-in provider

The built-in provider is a special provider that is always available to Pulumi
programs (see [](gh-file:pulumi#pkg/resource/deploy/builtins.go) for its
definition and [](gh-file:pulumi#pkg/resource/deploy/deployment.go#L489) for its
injection into deployments as part of the provider registry). It is used to
manage resources and functionality that are core to the Pulumi programming
model, such as [stack
references](https://www.pulumi.com/tutorials/building-with-pulumi/stack-references/)
and rehydrating [resource references](res-refs). It exposes the `pulumi` package
and provider instances thus belong to the package `pulumi:providers:pulumi`.
