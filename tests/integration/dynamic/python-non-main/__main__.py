# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from pulumi import export
from other import r

export("random_id", r.id)
export("random_val", r.val)
