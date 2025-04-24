import pulumi
import pulumi_ref_ref as ref_ref

res = ref_ref.Resource("res", data={
    "inner_data": {
        "boolean": False,
        "float": 2.17,
        "integer": -12,
        "string": "Goodbye",
        "bool_array": [
            False,
            True,
        ],
        "string_map": {
            "two": "turtle doves",
            "three": "french hens",
        },
    },
    "boolean": True,
    "float": 4.5,
    "integer": 1024,
    "string": "Hello",
    "bool_array": [],
    "string_map": {
        "x": "100",
        "y": "200",
    },
})
