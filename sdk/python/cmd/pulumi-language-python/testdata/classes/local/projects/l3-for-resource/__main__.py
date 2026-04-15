import pulumi
import pulumi_nestedobject as nestedobject

source = nestedobject.Container("source", inputs=[
    "a",
    "b",
    "c",
])
# for over list<object> output
receiver = nestedobject.Receiver("receiver", details=source.details.apply(lambda details: [{
    "key": detail.key,
    "value": detail.value,
} for detail in details]))
# for over list<string> output
from_simple = nestedobject.Container("fromSimple", inputs=source.details.apply(lambda details: [detail.value for detail in details]))
# for producing a map
mapped = nestedobject.MapContainer("mapped", tags=source.details.apply(lambda details: {detail.key: detail.value for detail in details}))
