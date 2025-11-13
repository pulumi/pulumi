import pulumi
import pulumi_ref_ref as ref_ref

# Check we can index into properties of objects returned in outputs, this is similar to ref-ref but 
# we index into the outputs
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
    "bool_array": [True],
    "string_map": {
        "x": "100",
        "y": "200",
    },
})
pulumi.export("bool", res.data.boolean)
pulumi.export("array", res.data.bool_array[0])
pulumi.export("map", res.data.string_map["x"])
pulumi.export("nested", res.data.inner_data.string_map["three"])
