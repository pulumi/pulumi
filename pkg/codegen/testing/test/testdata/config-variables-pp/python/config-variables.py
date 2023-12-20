import pulumi

config = pulumi.Config()
required_string = config.require("requiredString")
required_int = config.require_int("requiredInt")
required_float = config.require_float("requiredFloat")
required_bool = config.require_bool("requiredBool")
required_any = config.require_object("requiredAny")
optional_string = config.get("optionalString")
if optional_string is None:
    optional_string = "defaultStringValue"
optional_int = config.get_int("optionalInt")
if optional_int is None:
    optional_int = 42
optional_float = config.get_float("optionalFloat")
if optional_float is None:
    optional_float = 3.14
optional_bool = config.get_bool("optionalBool")
if optional_bool is None:
    optional_bool = True
optional_any = config.get_object("optionalAny")
if optional_any is None:
    optional_any = {
        "key": "value",
    }
