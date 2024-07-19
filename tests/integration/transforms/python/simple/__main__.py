# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import asyncio
from pulumi import Output, ComponentResource, ResourceOptions, ResourceTransformArgs, ResourceTransformResult, InvokeTransformArgs, InvokeTransformResult
from pulumi.runtime import register_stack_transform, register_invoke_transform
from random_ import Component, Random, Provider
from pulumi.runtime.sync_await import _sync_await

class MyComponent(ComponentResource):
    child: Random
    def __init__(self, name, opts = None):
        super().__init__("my:component:MyComponent", name, {}, opts)
        childOpts = ResourceOptions(parent=self,
                                    additional_secret_outputs=["length"])
        self.child = Random(f"{name}-child", 5, None, childOpts)
        self.register_outputs({})

# Scenario #1 - apply a transform to a CustomResource
def res1_transform(args: ResourceTransformArgs):
    print("res1 transform")
    return ResourceTransformResult(
        props=args.props,
        opts=ResourceOptions.merge(args.opts, ResourceOptions(
            additional_secret_outputs=["result"],
        ))
    )

res1 = Random(
    "res1",
    5,
    None,
    ResourceOptions(transforms=[res1_transform]))


# Scenario #2 - apply a transform to a Component to transform it's children
def res2_transform(args: ResourceTransformArgs):
    print("res2 transform")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformResult(
            props={ "prefix": "newDefault", **args.props },
            opts=ResourceOptions.merge(args.opts, ResourceOptions(
                additional_secret_outputs=["result"],
            )))

res2 = MyComponent(
    name="res2",
    opts=ResourceOptions(transforms=[res2_transform]))

# Scenario #3 - apply a transform to the Stack to transform all (future) resources in the stack
def res3_transform(args: ResourceTransformArgs):
    print("stack transform")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformResult(
            props={ **args.props, "prefix": "stackDefault" },
            opts=ResourceOptions.merge(args.opts, ResourceOptions(
                additional_secret_outputs=["result"],
            )))

register_stack_transform(res3_transform)

res3 = Random("res3", Output.secret(5))

# Scenario #4 - transforms are applied in order of decreasing specificity
# 1. (not in this example) Child transform
# 2. First parent transform
# 3. Second parent transform
# 4. Stack transform
def res4_transform_1(args: ResourceTransformArgs):
    print("res4 transform")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformResult(
            props={ **args.props, "prefix": "default1" },
            opts=args.opts)
def res4_transform_2(args: ResourceTransformArgs):
    print("res4 transform2")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformResult(
            props={ **args.props, "prefix": "default2" },
            opts=args.opts)

res4 = MyComponent(
    name="res4",
    opts=ResourceOptions(transforms=[
        res4_transform_1,
        res4_transform_2]))

# Scenario #5 - mutate the properties of a resource
def res5_transform(args: ResourceTransformArgs):
    print("res5 transform")
    if args.type_ == "testprovider:index:Random":
        length = args.props["length"]
        args.props["length"] = length * 2
        return ResourceTransformResult(
            props=args.props,
            opts=args.opts)

res5 = Random(
    "res5",
    10,
    None,
    ResourceOptions(transforms=[res5_transform]))

# Scenario #6 - mutate the provider on a custom resource
provider1 = Provider("provider1")
provider2 = Provider("provider2")

def provider_transform(args: ResourceTransformArgs):
    print("provider transform")
    return ResourceTransformResult(
        props=args.props,
        opts=ResourceOptions.merge(args.opts, ResourceOptions(
            provider=provider2,
        )))

res6 = Random(
    "res6",
    10,
    None,
    ResourceOptions(
        provider=provider1,
        transforms=[provider_transform],
    ))

# Scenario #7 - mutate the provider on a component resource
res7 = Component(
    "res7",
    10,
    ResourceOptions(
        provider=provider1,
        transforms=[provider_transform],
    ))

def res8_transform(args: InvokeTransformArgs):
    return InvokeTransformResult(
        args={ **args.args, 'length': 11 },
        opts=args.opts)

register_invoke_transform(res8_transform)

res8 = Random("res8", length=10)
args = {
    'length': 10,
    'prefix': "test",
}

result = _sync_await(res8.invoke(args))
if result["length"] != 11:
    raise Exception(f"expected length to be 11, got {result['length']}")
if result["prefix"] != "test":
    raise Exception(f"expected prefix to be test, got {result['prefix']}")
