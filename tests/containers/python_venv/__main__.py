import pulumi

config = pulumi.Config()
print("Hello from %s" % (config.require("runtime")))
