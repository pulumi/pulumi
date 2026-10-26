import pulumi
import pulumi_simple as simple
import pulumi_simple_invoke as simple_invoke

# Baseline for invoke dependency propagation: an invoke that reads properties from two different
# resources produces a return value whose consumer must depend on the union of both.
a = simple_invoke.StringResource("a", text="hello")
b = simple.Resource("b", value=True)
data = simple_invoke.secret_invoke_output(value=a.text,
    secret_response=b.value)
d = simple_invoke.StringResource("d", text=data.response)
