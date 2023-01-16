# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import pulumi

from fails_on_delete import FailsOnDelete
from random_ import Random

rand = Random("random", length=10)
FailsOnDelete("failsondelete", opts=pulumi.ResourceOptions(deleted_with=rand))
