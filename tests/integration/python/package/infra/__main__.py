# Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

"""An example program that use a module in the parent package to run"""

import helper

import pulumi

pulumi.export('foo', helper.value)
