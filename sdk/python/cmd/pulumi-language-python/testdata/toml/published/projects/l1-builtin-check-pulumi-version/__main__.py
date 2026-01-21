import pulumi

config = pulumi.Config()
version = config.require("version")
pulumi.check_pulumi_version(version)
