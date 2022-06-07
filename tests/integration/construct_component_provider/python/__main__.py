# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi


class Provider(pulumi.ProviderResource):
    message: pulumi.Output[str]

    def __init__(self, name: str, message: pulumi.Input[str], opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__("testcomponent", name, {"message": message}, opts)


class Component(pulumi.ComponentResource):
    message: pulumi.Output[str]

    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None) -> None:
        props = {
            "message": None
        }
        super().__init__("testcomponent:index:Component", name, props, opts, True)


component = Component("mycomponent", pulumi.ResourceOptions(
    providers={
        "testcomponent": Provider("myprovider", "hello world"),
    })
)


pulumi.export("message", component.message)
