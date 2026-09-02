# Copyright 2026, Pulumi Corporation.  All rights reserved.

import pulumi


# Motivating case: an AND reducer where True sticks to False. Once the reduced output becomes
# False, it cannot flip back to True even if the program's input is True again.
stash = pulumi.Stash(
    "bucket",
    input=True,
    reduce=lambda _old_input, old_output, new_input: bool(old_output) and bool(new_input),
)

pulumi.export("input", stash.input)
pulumi.export("output", stash.output)
