# Copyright 2024, Pulumi Corporation.

import pulumi.provider as provider
import sys
import os

class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name, resource_type, inputs, options):
        return provider.ConstructResult("", {"ITS_ALIVE": "IT'S ALIVE!"})

if __name__ == '__main__':
    provider.main(Provider(), sys.argv[1:])
