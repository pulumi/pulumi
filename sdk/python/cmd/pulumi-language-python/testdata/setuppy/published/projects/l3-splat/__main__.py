import pulumi
import pulumi_nestedobject as nestedobject
import pulumi_optional_primitive_ref as optional_primitive_ref

source = nestedobject.Container("source", inputs=[
    "a",
    "b",
])
sink = nestedobject.Container("sink", inputs=source.details.apply(lambda details: [__item.value for __item in details]))
set_optional = optional_primitive_ref.Resource("setOptional",
    data={},
    optional_data={
        "string": "set",
    })
unset_optional = optional_primitive_ref.Resource("unsetOptional", data={})
pulumi.export("setOptionalSplat", set_optional.optional_data.apply(lambda optional_data: [__item.string for __item in [__value for __value in [optional_data] if __value is not None]]))
pulumi.export("unsetOptionalSplat", unset_optional.optional_data.apply(lambda optional_data: [__item.string for __item in [__value for __value in [optional_data] if __value is not None]]))
