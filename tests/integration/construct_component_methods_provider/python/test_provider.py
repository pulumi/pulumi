# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import pulumi

class TestProvider(pulumi.ProviderResource):
    def __init__(__self__, resource_name: str):
        super().__init__("testprovider", resource_name)
