# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

"""An example program that type checks with pyright but pyright is not installed"""

import pulumi

# This export won't work because the first argument is a number, not a string
pulumi.export(42, 'bar')
