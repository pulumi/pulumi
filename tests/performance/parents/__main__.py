# Copyright 2024, Pulumi Corporation.

from pulumi import ComponentResource, ResourceOptions

from echo import Echo

parent =  Echo(f"echo-{0}", echo=f"hello-{0}")

for i in range(1, 100):
    e = Echo(f"echo-{i}", echo=f"hello-${i}", opts=ResourceOptions(parent=parent))
    parent = e
