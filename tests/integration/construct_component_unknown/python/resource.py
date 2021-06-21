# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Any, Optional

import pulumi
import pulumi.dynamic as dynamic


_ID = 0


class MyDynamicProvider(dynamic.ResourceProvider):
    def create(self, props: Any) -> dynamic.CreateResult:
        global _ID
        _ID = _ID + 1
        return dynamic.CreateResult(id_=str(_ID))


class Resource(dynamic.Resource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions]=None):
        super().__init__(MyDynamicProvider(), name, {}, opts)
