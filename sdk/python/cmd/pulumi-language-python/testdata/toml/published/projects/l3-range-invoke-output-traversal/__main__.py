import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

# A resource whose computed output feeds the invoke, forcing the invoke into its
# output-versioned form so that `values` is an Output.
source = nestedobject.Container("source", inputs=[
    "alpha",
    "bravo",
    "charlie",
])
values = nestedobject.get_values_output(names=source.inputs)
# Ranges over the length of the invoke's computed list and indexes that same
# Output-typed list by the loop counter inside the body. This is the shape from
# https://github.com/pulumi/pulumi/issues/12507.
routes: list[nestedobject.Target] = []
def create_routes(range_body):
    for routes_range in [{"value": i} for i in range(0, range_body)]:
        routes.append(nestedobject.Target(f"routes-{routes_range['value']}", name=values.apply(lambda values, _routes_range=routes_range: values.results[_routes_range["value"]])))

(pulumi.Output.from_input(values.results).apply(lambda value: len(value))).apply(create_routes)
