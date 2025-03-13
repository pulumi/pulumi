# Copyright 2016-2025, Pulumi Corporation.  All rights reserved.

import binascii
import os
from pulumi import export, ResourceOptions
from pulumi.dynamic import Resource, ResourceProvider, CreateResult, ReadResult

class RandomResourceProvider(ResourceProvider):
    def create(self, props):
        val = binascii.b2a_hex(os.urandom(15)).decode("ascii")
        return CreateResult(val, { "val": val })
    
    def read(self, id, props):
        props["val"] = id
        return ReadResult(id, props)

class Random(Resource):
    val: str
    def __init__(self, name, opts = None):
        super().__init__(RandomResourceProvider(), name, {"val": ""}, opts)

r = Random("foo")

export("foo_id", r.id)
export("foo_val", r.val)

s = Random("bar", ResourceOptions(import_="9db121f2bede6bd202f1556b841b78"))

export("bar_id", s.id)
export("bar_val", s.val)
