# Copyright 2025, Pulumi Corporation.  All rights reserved.

import pulumi
import some_package

pulumi.export("foo", some_package.double(3))
