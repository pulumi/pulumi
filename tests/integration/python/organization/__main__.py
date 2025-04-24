import pulumi

assert pulumi.get_organization() is not None, "Organization expected but not found"
