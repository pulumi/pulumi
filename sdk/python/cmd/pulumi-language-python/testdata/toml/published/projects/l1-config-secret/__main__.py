import pulumi

config = pulumi.Config()
a_number = config.require_secret_float("aNumber")
pulumi.export("theSecretNumber", a_number.apply(lambda a_number: a_number + 1.25))
