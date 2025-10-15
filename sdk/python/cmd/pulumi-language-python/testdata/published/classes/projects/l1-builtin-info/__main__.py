import pulumi

pulumi.export("stackOutput", pulumi.get_stack())
pulumi.export("projectOutput", pulumi.get_project())
pulumi.export("organizationOutput", pulumi.get_organization())
