#!/usr/bin/env python3

from typing import Any, Optional

from pulumi import Resource, export
from pulumi.resource import ProviderResource as Provider
from pulumi.resource import ResourceOptions


class Random(Resource):
    def __init__(self, name: str, length=int, opts: Optional[ResourceOptions]=None):
        self.length = length
        super().__init__("testprovider:index:Random", name, {"length": length}, opts)

class RandomProvider(Provider):
    def __init__(self, opts: Optional[ResourceOptions]=None):
        super().__init__("pulumi:providers:testprovider", "provider", None, opts)

r = Random("default", length=10, opts=ResourceOptions(plugin_download_url="example.com"))
provider = RandomProvider(ResourceOptions(plugin_download_url="get.pulumi/test/providers"))
e = Random("provided", length=8, opts=ResourceOptions(provider=provider))

export("default provider", r.result)
export("explicit provider", e.result)
