# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

"""An example program that type checks with mypy"""

import pulumi

# This export won't work because the first argument is a number, not a string
pulumi.export(42, 'bar')
