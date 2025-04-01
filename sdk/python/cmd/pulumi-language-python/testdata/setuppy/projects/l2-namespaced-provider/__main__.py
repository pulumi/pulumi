import pulumi
import a_namespace_namespaced as namespaced

res = namespaced.Resource("res", value=True)
