import pulumi
import json

config = pulumi.Config()
a_string = config.require("aString")
a_number = config.require_float("aNumber")
a_list = config.require_object("aList")
a_secret = config.require_secret("aSecret")
# Literal data shapes built as locals
literal_bool = True
literal_array = [
    "x",
    "y",
    "z",
]
literal_object = {
    "key": "value",
    "count": 1,
}
# Nested object using config values
nested_object = {
    "name": a_string,
    "items": a_list,
    "a_secret": a_secret,
}
pulumi.export("stringOutput", json.dumps(a_string))
pulumi.export("numberOutput", json.dumps(a_number))
pulumi.export("boolOutput", json.dumps(literal_bool))
pulumi.export("arrayOutput", json.dumps(literal_array))
pulumi.export("objectOutput", json.dumps(literal_object))
pulumi.export("nestedOutput", pulumi.Output.json_dumps(nested_object))
