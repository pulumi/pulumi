import pulumi
import pulumi_any_handled as any_handled

a_string = any_handled.Resource("aString", value="a string")
a_boolean = any_handled.Resource("aBoolean", value=True)
a_number = any_handled.Resource("aNumber", value=42)
a_list = any_handled.Resource("aList", value=[
    1,
    True,
    "three",
])
an_object = any_handled.Resource("anObject", value={
    "key": "value",
    "nested": {
        "count": 1,
    },
})
an_asset = any_handled.Resource("anAsset", value=pulumi.StringAsset("the asset contents"))
