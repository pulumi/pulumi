import pulumi
import pulumi_simple as simple
import pulumi_simple_invoke as simple_invoke

first = simple.Resource("first", value=False)
# assert that resource second depends on resource first
# because it uses .secret from the invoke which depends on first
second = simple.Resource("second", value=simple_invoke.secret_invoke_output(value="hello",
    secret_response=first.value).apply(lambda invoke: invoke.secret))
