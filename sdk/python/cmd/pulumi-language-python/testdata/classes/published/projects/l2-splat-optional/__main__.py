import pulumi
import pulumi_splatoptional as splatoptional

source = splatoptional.Container("source", inputs=[
    "a",
    "b",
])
sink = splatoptional.Container("sink", inputs=source.details.apply(lambda details: [__item.value for __item in details]).apply(lambda x: [item for item in x if item is not None]))
pulumi.export("values", source.details.apply(lambda details: [__item.value for __item in details]))
