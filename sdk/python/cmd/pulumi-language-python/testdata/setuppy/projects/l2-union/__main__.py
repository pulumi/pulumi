import pulumi
import pulumi_union as union

string_or_integer_example1 = union.Example("stringOrIntegerExample1", string_or_integer_property=42)
string_or_integer_example2 = union.Example("stringOrIntegerExample2", string_or_integer_property="fourty two")
map_map_union_example = union.Example("mapMapUnionExample", map_map_union_property={
    "key1": {
        "key1a": "value1a",
    },
})
pulumi.export("mapMapUnionOutput", map_map_union_example.map_map_union_property)
