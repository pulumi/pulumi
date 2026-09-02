# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


def reduce(_old_input, old_output, new_input):
    # On create old_output is None; treat that as the identity for AND (seed from new_input).
    if old_output is None:
        return new_input
    return bool(old_output) and bool(new_input)


# Motivating case: an AND reducer where True sticks to False. Once the reduced output becomes
# False, it cannot flip back to True even if the program's input is True again.
stash = pulumi.Stash("bucket", input=True, reduce=reduce)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
