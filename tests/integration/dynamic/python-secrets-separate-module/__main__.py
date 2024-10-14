# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi

from simple_provider import SimpleResource, SimpleResourceWithConfig


r_with_config = SimpleResourceWithConfig("with-config")
pulumi.export("authenticated_with_config", r_with_config.authenticated)

r_without_config = SimpleResource("without-config")
pulumi.export("authenticated_without_config", r_without_config.authenticated)
