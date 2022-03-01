# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi

from component import Component

component = Component("component", first="Hello", second="World")
result = component.get_message("Alice")

pulumi.export("message", result.message)
