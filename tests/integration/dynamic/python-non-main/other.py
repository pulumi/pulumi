# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import binascii
import os
from pulumi.dynamic import Resource, ResourceProvider, CreateResult

# Define the dynamic provider and create the dynamic provider resource from a
# file that is not `__main__` per pulumi/pulumi#7453.

class RandomResourceProvider(ResourceProvider):
    def create(self, props):
        val = binascii.b2a_hex(os.urandom(15)).decode("ascii")
        return CreateResult(val, { "val": val })

class Random(Resource):
    val: str
    def __init__(self, name, opts = None):
        super().__init__(RandomResourceProvider(), name, {"val": ""}, opts)

r = Random("foo")