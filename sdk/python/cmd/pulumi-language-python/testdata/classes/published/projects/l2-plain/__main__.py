import pulumi
import pulumi_plain as plain

res = plain.Resource("res", data=plain.DataArgs(
    inner_data=plain.InnerDataArgs(
        boolean=False,
        float=2.17,
        integer=-12,
        string="Goodbye",
        bool_array=[
            False,
            True,
        ],
        string_map={
            "two": "turtle doves",
            "three": "french hens",
        },
    ),
    boolean=True,
    float=4.5,
    integer=1024,
    string="Hello",
    bool_array=[
        True,
        False,
    ],
    string_map={
        "x": "100",
        "y": "200",
    },
))
