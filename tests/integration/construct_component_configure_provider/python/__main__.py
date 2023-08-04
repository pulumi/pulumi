# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import pulumi
import pulumi_metaprovider
import pulumi_tls
import helpers

config = pulumi.Config()
proxy = config.require('proxy')

configurer = pulumi_metaprovider.Configurer("configurer", tls_proxy=helpers.unknownIfDryRun(proxy))
tls_provider = configurer.tls_provider()

key = pulumi_tls.PrivateKey("my-private-key",
                            algorithm="ECDSA",
                            ecdsa_curve="P384",
                            opts=pulumi.ResourceOptions(provider=tls_provider))

pulumi.export("keyAlgo", key.algorithm)
pulumi.export("meaningOfLife", configurer.meaning_of_life() + 1 - 1)
