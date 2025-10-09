import pulumi
import pulumi_plain as plain
import pulumi_primitive as primitive
import pulumi_primitive_ref as primitive_ref
import pulumi_ref_ref as ref_ref

prim = primitive.Resource("prim",
    boolean=False,
    float=2.17,
    integer=-12,
    string="Goodbye",
    number_array=[
        0,
        1,
    ],
    boolean_map={
        "my key": False,
        "my.key": True,
        "my-key": False,
        "my_key": True,
        "MY_KEY": False,
        "myKey": True,
    })
ref = primitive_ref.Resource("ref", data={
    "boolean": False,
    "float": 2.17,
    "integer": -12,
    "string": "Goodbye",
    "bool_array": [
        False,
        True,
    ],
    "string_map": {
        "my key": "one",
        "my.key": "two",
        "my-key": "three",
        "my_key": "four",
        "MY_KEY": "five",
        "myKey": "six",
    },
})
rref = ref_ref.Resource("rref", data={
    "inner_data": {
        "boolean": False,
        "float": -2.17,
        "integer": 123,
        "string": "Goodbye",
        "bool_array": [],
        "string_map": {
            "my key": "one",
            "my.key": "two",
            "my-key": "three",
            "my_key": "four",
            "MY_KEY": "five",
            "myKey": "six",
        },
    },
    "boolean": True,
    "float": 4.5,
    "integer": 1024,
    "string": "Hello",
    "bool_array": [],
    "string_map": {
        "my key": "one",
        "my.key": "two",
        "my-key": "three",
        "my_key": "four",
        "MY_KEY": "five",
        "myKey": "six",
    },
})
plains = plain.Resource("plains",
    data={
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
                "my key": "one",
                "my.key": "two",
                "my-key": "three",
                "my_key": "four",
                "MY_KEY": "five",
                "myKey": "six",
            },
        },
        "boolean": True,
        "float": 4.5,
        "integer": 1024,
        "string": "Hello",
        "bool_array": [
            True,
            False,
        ],
        "string_map": {
            "my key": "one",
            "my.key": "two",
            "my-key": "three",
            "my_key": "four",
            "MY_KEY": "five",
            "myKey": "six",
        },
    },
    non_plain_data={
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
                "my key": "one",
                "my.key": "two",
                "my-key": "three",
                "my_key": "four",
                "MY_KEY": "five",
                "myKey": "six",
            },
        },
        "boolean": True,
        "float": 4.5,
        "integer": 1024,
        "string": "Hello",
        "bool_array": [
            True,
            False,
        ],
        "string_map": {
            "my key": "one",
            "my.key": "two",
            "my-key": "three",
            "my_key": "four",
            "MY_KEY": "five",
            "myKey": "six",
        },
    })
