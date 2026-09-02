# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


def reduce(_old_input, old_output, new_input):
    if old_output is None:
        return new_input
    return bool(old_output) and bool(new_input)


# Once the reduced output is False, flipping input back to True must not resurrect it:
# reducer(False, False, True) == False and True == False.
stash = pulumi.Stash("bucket", input=True, reduce=reduce)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
