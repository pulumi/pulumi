# Copyright 2025, Pulumi Corporation.

import pulumi

config = pulumi.Config()
stack = config.require("stack")

ref = pulumi.StackReference("stack", stack)

pulumi.export("echo-1", ref.get_output("echo-1"))
