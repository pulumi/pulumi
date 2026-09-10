import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

container = nestedobject.Container("container", inputs=[
    "alpha",
    "bravo",
])
map_container = nestedobject.MapContainer("mapContainer", tags={
    "k1": "charlie",
    "k2": "delta",
})
# A resource that ranges over a computed list
list_output: list[nestedobject.Target] = []
def create_list_output(range_body):
    for list_output_range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        list_output.append(nestedobject.Target(f"listOutput-{list_output_range['key']}", name=list_output_range["value"].value))

container.details.apply(create_list_output)
# A resource that ranges over a computed map
map_output: dict[str, nestedobject.Target] = {}
def create_map_output(range_body):
    for map_output_range in [{"key": k, "value": v} for [k, v] in sorted((range_body).items())]:
        map_output[map_output_range['key']] = nestedobject.Target(f"mapOutput-{map_output_range['key']}", name=f"{map_output_range['key']}=>{map_output_range['value']}")

map_container.tags.apply(create_map_output)
