# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


# Once the reduced output is False, flipping input back to True must not resurrect it:
# reducer(False, False, True) == False and True == False.
stash = pulumi.Stash(
    "bucket",
    input=True,
    reduce=lambda _old_input, old_output, new_input: bool(old_output) and bool(new_input),
)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
