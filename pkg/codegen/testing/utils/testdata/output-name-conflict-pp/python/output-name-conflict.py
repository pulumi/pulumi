import pulumi

config = pulumi.Config()
cidr_block = config.get("cidrBlock")
if cidr_block is None:
    cidr_block = "Test config variable"
pulumi.export("cidrBlock", cidr_block)
