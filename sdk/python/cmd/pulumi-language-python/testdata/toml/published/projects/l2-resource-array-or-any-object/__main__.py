import pulumi
import pulumi_array_or_any as array_or_any

object_list = array_or_any.ObjectList("objectList", values=[
    {
        "itemName": "first",
    },
    {
        "itemName": "second",
    },
])
object_any = array_or_any.ObjectList("objectAny", values={
    "any": True,
})
