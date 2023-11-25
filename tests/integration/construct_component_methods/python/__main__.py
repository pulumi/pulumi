# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi

from component import Component

component = Component("component", first="Hello", second="World")
result = component.get_message("Alice")
message = result.message

pulumi.export("message", message)

async def get_deps(o: pulumi.Output[str]):
    resources = await o.resources()
    return [await res.urn.future() for res in resources]

pulumi.export("messagedeps", get_deps(message))
