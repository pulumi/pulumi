# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import asyncio
from pulumi import Output, ComponentResource, ResourceOptions, ResourceTransformationArgs, ResourceTransformationResult
from pulumi.runtime import register_stack_transformation
from random_ import Random

class MyComponent(ComponentResource):
    child: Random
    def __init__(self, name, opts = None):
        super().__init__("my:component:MyComponent", name, {}, opts)
        childOpts = ResourceOptions(parent=self,
                                    additional_secret_outputs=["length"])
        self.child = Random(f"{name}-child", 5, None, childOpts)
        self.register_outputs({})

# Scenario #1 - apply a transformation to a CustomResource
def res1_transformation(args: ResourceTransformationArgs):
    print("res1 transformation")
    return ResourceTransformationResult(
        props=args.props,
        opts=ResourceOptions.merge(args.opts, ResourceOptions(
            additional_secret_outputs=["result"],
        ))
    )

res1 = Random(
    "res1",
    5,
    None,
    ResourceOptions(transformations=[res1_transformation]))


# Scenario #2 - apply a transformation to a Component to transform it's children
def res2_transformation(args: ResourceTransformationArgs):
    print("res2 transformation")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformationResult(
            props={ "prefix": "newDefault", **args.props },
            opts=ResourceOptions.merge(args.opts, ResourceOptions(
                additional_secret_outputs=["result"],
            )))

res2 = MyComponent(
    name="res2",
    opts=ResourceOptions(transformations=[res2_transformation]))

# Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
def res3_transformation(args: ResourceTransformationArgs):
    print("stack transformation")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformationResult(
            props={ **args.props, "prefix": "stackDefault" },
            opts=ResourceOptions.merge(args.opts, ResourceOptions(
                additional_secret_outputs=["result"],
            )))

register_stack_transformation(res3_transformation)

res3 = Random("res3", 5)

# Scenario #4 - transformations are applied in order of decreasing specificity
# 1. (not in this example) Child transformation
# 2. First parent transformation
# 3. Second parent transformation
# 4. Stack transformation
def res4_transformation_1(args: ResourceTransformationArgs):
    print("res4 transformation")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformationResult(
            props={ **args.props, "prefix": "default1" },
            opts=args.opts)
def res4_transformation_2(args: ResourceTransformationArgs):
    print("res4 transformation2")
    if args.type_ == "testprovider:index:Random":
        return ResourceTransformationResult(
            props={ **args.props, "prefix": "default2" },
            opts=args.opts)

res4 = MyComponent(
    name="res4",
    opts=ResourceOptions(transformations=[
        res4_transformation_1,
        res4_transformation_2]))

# Scenario #5 - cross-resource transformations that inject dependencies on one resource into another.

class MyOtherComponent(ComponentResource):
    child1: Random
    child2: Random
    def __init__(self, name, opts = None):
        super().__init__("my:component:MyOtherComponent", name, {}, opts)
        self.child = Random(f"{name}-child1", 5, None, ResourceOptions(parent=self))
        self.child = Random(f"{name}-child2", 5, None, ResourceOptions(parent=self))
        self.register_outputs({})

def transform_child1_depends_on_child2():
    # Create a future that wil be resolved once we find child2.  This is needed because we do not
    # know what order we will see the resource registrations of child1 and child2.
    child2_future = asyncio.Future()
    def transform(args: ResourceTransformationArgs):
        print("res5 transformation")
        if args.name.endswith("-child2"):
            # Resolve the child2 promise with the child2 resource.
            child2_future.set_result(args.resource)
            return None
        elif args.name.endswith("-child1"):
            # Overwrite the `prefix` to child2 with a dependency on the `result` from child1.
            async def getResult(input):
                if input != 5:
                    # Not strictly necessary - but shows we can confirm invariants we expect to be
                    # true.
                    raise Exception("unexpected input value", input)
                child2 = await child2_future
                return child2.result
            child2_result = Output.from_input(args.props["length"]).apply(getResult)
            # Finally - overwrite the input of child2.
            return ResourceTransformationResult(
                props={ **args.props, "prefix": child2_result },
                opts=args.opts)
    return transform

res5 = MyOtherComponent(
    name="res5",
    opts=ResourceOptions(transformations=[transform_child1_depends_on_child2()]))
