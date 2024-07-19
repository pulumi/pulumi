# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi

class Random(pulumi.CustomResource):
    def __init__(self,
                 resource_name: str,
                 length: pulumi.Input[int],
                 prefix: Optional[pulumi.Input[str]] = None,
                 opts: Optional[pulumi.ResourceOptions] = None):
        props = {
            "length": length,
            "result": None,
            "prefix": prefix,
        }
        super().__init__("testprovider:index:Random", resource_name, props, opts)

    @property
    @pulumi.getter
    def length(self) -> pulumi.Output[int]:
        return pulumi.get(self, "length")

    @property
    @pulumi.getter
    def result(self) -> pulumi.Output[str]:
        return pulumi.get(self, "result")

    def invoke(self, args):
        return pulumi.runtime.invoke("testprovider:index:returnArgs", args)


class Component(pulumi.ComponentResource):
    def __init__(self,
                 resource_name: str,
                 length: pulumi.Input[int],
                 opts: Optional[pulumi.ResourceOptions] = None):
        props = {
            "length": length,
            "childId": None,
        }
        super().__init__("testprovider:index:Component", resource_name, props, opts, True)

    @property
    @pulumi.getter
    def length(self) -> pulumi.Output[int]:
        return pulumi.get(self, "length")

    @property
    @pulumi.getter
    def child_id(self) -> pulumi.Output[str]:
        return pulumi.get(self, "childId")

class Provider(pulumi.ProviderResource):
    def __init__(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None):
        super(Provider, __self__).__init__(
            'testprovider',
            resource_name,
            None,
            opts)
