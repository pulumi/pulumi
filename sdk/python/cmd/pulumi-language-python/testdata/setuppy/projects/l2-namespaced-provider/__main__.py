import pulumi
import a_namespace_namespaced as namespaced
import pulumi_simple as simple

simple_res = simple.Resource("simpleRes", value=True)
res = namespaced.Resource("res", value=True)
