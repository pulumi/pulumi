import pulumi

config = pulumi.Config()
version = config.require("version")
pulumi.require_pulumi_version(version)
