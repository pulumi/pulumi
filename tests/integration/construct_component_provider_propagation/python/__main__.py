# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi

class Component(pulumi.ComponentResource):
    """
    Python-level remote component for the component resource
    defined in sibling testcomponent-go directory.
    """

    result: pulumi.Output[str]

    def __init__(self,
                 name: str,
                 opts: Optional[pulumi.ResourceOptions] = None):
        props = {"result": None}
        super().__init__("testcomponent:index:Component", name, props, opts, remote=True)


class RandomProvider(pulumi.ProviderResource):
    """
    Provider for the testprovider:index:Random resource.

    Implemented in tests/testprovider.
    """

    def __init__(self, name, opts: Optional[pulumi.ResourceOptions]=None):
        super().__init__("testprovider", name, {}, opts)


explicit_provider = RandomProvider("explicit")

# Should pick up the default provider.
Component("uses_default")

# Should use the provider passed in as an argument.
Component("uses_provider", opts=pulumi.ResourceOptions(
    provider=explicit_provider,
))

# Should use the provider passed in as an argument
Component("uses_providers", opts=pulumi.ResourceOptions(
    providers=[explicit_provider],
))

# Should use the provider passed in as an argument
Component("uses_providers_map", opts=pulumi.ResourceOptions(
    providers={"testprovider": explicit_provider},
))
