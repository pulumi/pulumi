# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import pulumi

config = pulumi.Config('config_missing_py')
config.require_secret('notFound')