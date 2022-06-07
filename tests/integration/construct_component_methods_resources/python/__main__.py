# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi

from component import Component

component = Component("component")
result = component.create_random(length=10).result

pulumi.export("result", result)
