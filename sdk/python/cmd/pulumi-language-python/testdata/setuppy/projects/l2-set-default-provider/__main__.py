import pulumi
import pulumi_simple as simple

simple_provider = simple.Provider("simple_provider")
with pulumi.default_providers([simple_provider]):
    non_default_resource = simple.Resource("non_default_resource", value=True)
default_resource = simple.Resource("default_resource", value=True)
