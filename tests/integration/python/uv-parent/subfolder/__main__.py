# Copyright 2024, Pulumi Corporation.  All rights reserved.

"""A Python project that uses the uv toolchain where the pyproject.toml is in a parent folder."""

import pulumi

pulumi.export("foo", "bar")
