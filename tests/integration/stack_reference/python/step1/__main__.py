# Copyright 2020, Pulumi Corporation.  All rights reserved.

import pulumi

slug = f"{pulumi.get_organization()}/{pulumi.get_project()}/{pulumi.get_stack()}"
a = pulumi.StackReference(slug)

oldVal = pulumi.runtime.sync_await(a.get_output_details('val')).value

if len(oldVal) != 2 or oldVal[0] != 'a' or oldVal[1] != 'b':
    raise Exception('Invalid result')

pulumi.export('val2', pulumi.Output.secret(['a', 'b']))