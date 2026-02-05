import pulumi

config = pulumi.Config()
a_number = config.require_float("aNumber")
pulumi.export("theNumber", a_number + 1.25)
a_string = config.require("aString")
pulumi.export("theString", f"{a_string} World")
a_bool = config.require_bool("aBool")
pulumi.export("theBool", not a_bool and True)
