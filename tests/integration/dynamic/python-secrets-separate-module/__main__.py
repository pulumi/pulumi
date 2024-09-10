# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi

from simple_provider import SimpleResource


r = SimpleResource("foo")
pulumi.export("out", r.authenticated)
