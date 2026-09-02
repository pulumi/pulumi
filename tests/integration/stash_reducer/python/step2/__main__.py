# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


# input flips True -> False. reducer(True, True, False) == True and False == False, so the
# stash output transitions to False.
stash = pulumi.Stash(
    "bucket",
    input=False,
    reduce=lambda _old_input, old_output, new_input: bool(old_output) and bool(new_input),
)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
