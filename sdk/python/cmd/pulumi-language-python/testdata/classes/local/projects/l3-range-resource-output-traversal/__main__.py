import pulumi
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
list_output = []
def create_list_output(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        list_output.append(nestedobject.Target(f"listOutput-{range['key']}", name=range["value"].value))

container.details.apply(create_list_output)
# A resource that ranges over a computed map
map_output = []
def create_map_output(range_body):
    for range in [{"key": k, "value": v} for [k, v] in sorted((range_body).items())]:
        map_output.append(nestedobject.Target(f"mapOutput-{range['key']}", name=f"{range['key']}=>{range['value']}"))

map_container.tags.apply(create_map_output)
