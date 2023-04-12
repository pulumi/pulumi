# Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi


class Child(pulumi.ComponentResource):
    @pulumi.input_type
    class ChildArgs:
        pass

    def __init__(
        self,
        resource_name: str,
        message: Optional[str] = None,
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        props = Container.ContainerArgs.__new__(Container.ContainerArgs)
        props.__dict__["message"] = message
        super().__init__("test:index:Child", resource_name, props, opts)
        if opts and opts.urn:
            return
        self.register_outputs({ "message": message })

    @property
    @pulumi.getter
    def message(self) -> pulumi.Output[str]:
        return pulumi.get(self, "message")


class Container(pulumi.ComponentResource):
    @pulumi.input_type
    class ContainerArgs:
        pass

    def __init__(
        self,
        resource_name: str,
        child: Optional[Child] = None,
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        props = Container.ContainerArgs.__new__(Container.ContainerArgs)
        props.__dict__["child"] = child
        super().__init__("test:index:Container", resource_name, props, opts)
        if opts and opts.urn:
            return
        self.register_outputs({ "child": child })

    @property
    @pulumi.getter
    def child(self) -> pulumi.Output[Child]:
        return pulumi.get(self, "child")


class Module(pulumi.runtime.ResourceModule):
    def version(self):
        return "0.0.1"

    def construct(self, name: str, typ: str, urn: str) -> pulumi.Resource:
        if typ == "test:index:Child":
            return Child(name, opts=pulumi.ResourceOptions(urn=urn))
        else:
            raise Exception(f"unknown resource type {typ}")


pulumi.runtime.register_resource_module("test", "index", Module())


child = Child("mychild", message="hello world!")
container = Container("mycontainer", child=child)


def assert_equal(args):
    expected_urn = args["expected_urn"]
    actual_urn = args["actual_urn"]
    actual_message = args["actual_message"]
    assert expected_urn == actual_urn, \
        f"expected urn '{expected_urn}' not equal to actual urn '{actual_urn}'"
    assert "hello world!" == actual_message, \
        f"expected message 'hello world!' not equal to actual message '{actual_message}'"


def round_trip(urn: str):
    round_tripped_container = Container("mycontainer", opts=pulumi.ResourceOptions(urn=urn))
    pulumi.Output.all(
        expected_urn=child.urn,
        actual_urn=round_tripped_container.child.urn,
        actual_message=round_tripped_container.child.message,
    ).apply(assert_equal)


container.urn.apply(round_trip)
