# Copyright 2024, Pulumi Corporation.

from pulumi import Output, export

for i in range(1, 100):
    export(f"echo-{i}", Output.secret(Output.from_input(i)))
