import pulumi
import a_namespace_namespaced as namespaced
import pulumi_component as component

component_res = component.ComponentCustomRefOutput("componentRes", value="foo-bar-baz")
res = namespaced.Resource("res",
    value=True,
    resource_ref=component_res.ref)
