# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

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


class LocalComponent(pulumi.ComponentResource):
    message: pulumi.Output[str]

    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None) -> None:
        super().__init__("my:index:LocalComponent", name, {}, opts)

        component = Component(f"{name}-mycomponent", pulumi.ResourceOptions(parent=self))
        self.message = component.message


provider = Provider("myprovider", "hello world")
component = Component("mycomponent", pulumi.ResourceOptions(
    provider=provider,
))
localComponent = LocalComponent("mylocalcomponent", pulumi.ResourceOptions(
    providers=[provider],
))

pulumi.export("message", component.message)
pulumi.export("nestedMessage", localComponent.message)
