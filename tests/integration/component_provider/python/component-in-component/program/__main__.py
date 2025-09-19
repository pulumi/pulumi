import pulumi
import pulumi_provider as provider

comp = provider.MyComponent("comp")

pulumi.export("str_output", comp.str_output)
