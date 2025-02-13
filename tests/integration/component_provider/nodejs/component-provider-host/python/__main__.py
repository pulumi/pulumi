import pulumi
import pulumi_nodejs_component_provider as provider

comp = provider.MyComponent("comp")

pulumi.export("urn", comp.urn)
