import pulumi
import pulumi_nestedobject as nestedobject

source = nestedobject.Container("source", inputs=[
    "a",
    "b",
])
sink = nestedobject.Container("sink", inputs=source.details.apply(lambda details: [__item.value for __item in details]))
