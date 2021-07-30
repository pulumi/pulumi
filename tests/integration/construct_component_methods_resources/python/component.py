# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi

class Component(pulumi.ComponentResource):
    def __init__(self,
                 name: str,
                 opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("testcomponent:index:Component", name, {}, opts, True)

    @pulumi.output_type
    class CreateRandomResult:
        def __init__(self, result: str):
            if result and not isinstance(result, str):
                raise TypeError("Expected argument 'result' to be a str")
            pulumi.set(self, "result", result)

        @property
        @pulumi.getter
        def result(self) -> str:
            return pulumi.get(self, "result")

    def create_random(__self__, length: pulumi.Input[int]) -> pulumi.Output['Component.CreateRandomResult']:
        __args__ = dict()
        __args__['__self__'] = __self__
        __args__['length'] = length
        return pulumi.runtime.call('testcomponent:index:Component/createRandom',
                                   __args__,
                                   res=__self__,
                                   typ=Component.CreateRandomResult)
