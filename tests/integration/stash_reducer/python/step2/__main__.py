# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


def reduce(_old_input, old_output, new_input):
    if old_output is None:
        return new_input
    return bool(old_output) and bool(new_input)


# input flips True -> False. reducer(True, True, False) == True and False == False, so the
# stash output transitions to False.
stash = pulumi.Stash("bucket", input=False, reduce=reduce)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
