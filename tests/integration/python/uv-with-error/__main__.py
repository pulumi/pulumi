# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi_random as random

r = random.RandomString("random", length=16)

# This will fail because the name is already used
r2 = random.RandomString("random", length=16)
