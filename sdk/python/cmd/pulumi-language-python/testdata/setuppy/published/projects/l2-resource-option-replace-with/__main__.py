import pulumi
import pulumi_simple as simple

target = simple.Resource("target", value=True)
replace_with = simple.Resource("replaceWith", value=True,
opts = pulumi.ResourceOptions(replace_with=[target]))
not_replace_with = simple.Resource("notReplaceWith", value=True)
