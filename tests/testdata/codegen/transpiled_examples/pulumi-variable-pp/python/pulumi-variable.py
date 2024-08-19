import pulumi
import os

pulumi.export("cwd", os.getcwd())
pulumi.export("stack", pulumi.get_stack())
pulumi.export("project", pulumi.get_project())
pulumi.export("organization", pulumi.get_organization())
