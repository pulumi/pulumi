import pulumi

config = pulumi.Config()
a_number = config.require_float("aNumber")
pulumi.export("theNumber", a_number + 1.25)
optional_number = config.get_float("optionalNumber")
if optional_number is None:
    optional_number = 41.5
pulumi.export("defaultNumber", optional_number + 1.2)
an_int = config.require_int("anInt")
pulumi.export("theInteger", an_int + 4)
optional_int = config.get_int("optionalInt")
if optional_int is None:
    optional_int = 1
pulumi.export("defaultInteger", optional_int + 2)
a_string = config.require("aString")
pulumi.export("theString", f"{a_string} World")
optional_string = config.get("optionalString")
if optional_string is None:
    optional_string = "defaultStringValue"
pulumi.export("defaultString", optional_string)
a_bool = config.require_bool("aBool")
pulumi.export("theBool", not a_bool and True)
optional_bool = config.get_bool("optionalBool")
if optional_bool is None:
    optional_bool = False
pulumi.export("defaultBool", optional_bool)
