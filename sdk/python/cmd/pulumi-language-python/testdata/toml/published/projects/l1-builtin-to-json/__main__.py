import pulumi
import json

config = pulumi.Config()
a_string = config.require("aString")
a_number = config.require_float("aNumber")
a_list = config.require_object("aList")
a_secret = config.require_secret("aSecret")
pulumi.export("stringOutput", json.dumps(a_string))
pulumi.export("numberOutput", json.dumps(a_number))
pulumi.export("boolOutput", json.dumps(True))
pulumi.export("arrayOutput", json.dumps([
    "x",
    "y",
    "z",
]))
pulumi.export("objectOutput", json.dumps({
    "key": "value",
    "count": 1,
}))
# Nested object using config values
nested_object = {
    "anObject": {
        "name": a_string,
        "items": a_list,
    },
    "a_secret": a_secret,
}
pulumi.export("nestedOutput", pulumi.Output.json_dumps(nested_object))
