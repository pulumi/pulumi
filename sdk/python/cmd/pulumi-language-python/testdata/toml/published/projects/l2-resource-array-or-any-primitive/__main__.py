import pulumi
import pulumi_array_or_any as array_or_any

string_list = array_or_any.StringList("stringList", values=[
    "a",
    "b",
    "c",
])
string_any = array_or_any.StringList("stringAny", values={
    "any": True,
})
