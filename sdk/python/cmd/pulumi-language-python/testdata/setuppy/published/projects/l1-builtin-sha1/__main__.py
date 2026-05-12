import pulumi
import hashlib

config = pulumi.Config()
input = config.require("input")
hash = hashlib.sha1(input.encode()).hexdigest()
pulumi.export("hash", hash)
