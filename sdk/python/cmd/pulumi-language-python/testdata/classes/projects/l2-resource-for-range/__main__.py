import pulumi
from submodule import Submodule
import pulumi_simple as simple

config = pulumi.Config()
list_var = config.get_object("listVar")
if list_var is None:
    list_var = [
        "one",
        "two",
        "three",
    ]
filter_cond = config.get_bool("filterCond")
if filter_cond is None:
    filter_cond = True
res = []
for range in [{"key": k, "value": v} for [k, v] in enumerate({k: v for k, v in list_var if filter_cond})]:
    res.append(simple.Resource(f"res-{range['key']}", value=True))
eventual_list_var = pulumi.Output.secret(list_var)
eventual_res = []
def create_eventual_res(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        eventual_res.append(simple.Resource(f"eventualRes-{range['key']}", value=True))

eventual_list_var.apply(lambda resolved_outputs: create_eventual_res({k: v for k, v in resolved_outputs['eventual_list_var'] if filter_cond}))
submodule_comp = Submodule("submoduleComp", {
    'submoduleListVar': ["one"], 
    'submoduleFilterCond': True, 
    'submoduleFilterVariable': 1})
