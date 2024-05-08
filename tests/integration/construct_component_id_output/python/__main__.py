# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import pulumi
from component import Component

component_a = Component("a", id="hello")

pulumi.export("id", component_a.id)