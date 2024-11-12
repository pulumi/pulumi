# Copyright 2024, Pulumi Corporation.  All rights reserved.

"""A Python project that uses the uv toolchain where the requirements.txt is in a parent folder."""

import pulumi

pulumi.export("foo", "bar")
