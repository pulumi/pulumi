import pulumi
import pulumi_union as union

string_or_integer_example1 = union.Example("stringOrIntegerExample1", string_or_integer_property=42)
string_or_integer_example2 = union.Example("stringOrIntegerExample2", string_or_integer_property="forty two")
map_map_union_example = union.Example("mapMapUnionExample", map_map_union_property={
    "key1": {
        "key1a": "value1a",
    },
})
pulumi.export("mapMapUnionOutput", map_map_union_example.map_map_union_property)
# List<Union<String, Enum>> pattern
string_enum_union_list_example = union.Example("stringEnumUnionListExample", string_enum_union_list_property=[
    union.AccessRights.LISTEN,
    union.AccessRights.SEND,
    "NotAnEnumValue",
])
# Safe enum: literal string matching an enum value
safe_enum_example = union.Example("safeEnumExample", typed_enum_property=union.BlobType.BLOCK)
# Output enum: output from another resource used as enum input
enum_output_example = union.EnumOutput("enumOutputExample", name="example")
output_enum_example = union.Example("outputEnumExample", typed_enum_property=enum_output_example.type)
