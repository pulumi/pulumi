import pulumi
import pulumi_nestedobject as nestedobject

container = nestedobject.Container("container", inputs=[
    "alpha",
    "bravo",
])
target = []
def create_target(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        target.append(nestedobject.Target(f"target-{range['key']}", name=range["value"].value))

container.details.apply(create_target)
