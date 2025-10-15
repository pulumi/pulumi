import pulumi
import pulumi_keywords as keywords

first_resource = keywords.SomeResource("firstResource",
    builtins="builtins",
    property="property")
second_resource = keywords.SomeResource("secondResource",
    builtins=first_resource.builtins,
    property=first_resource.property)
