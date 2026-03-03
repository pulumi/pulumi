import pulumi

config = pulumi.Config()
a_secret = config.require_secret("aSecret")
not_secret = config.require("notSecret")
pulumi.export("roundtripSecret", a_secret)
pulumi.export("roundtripNotSecret", not_secret)
pulumi.export("open", pulumi.Output.unsecret(a_secret))
pulumi.export("close", pulumi.Output.secret(not_secret))
