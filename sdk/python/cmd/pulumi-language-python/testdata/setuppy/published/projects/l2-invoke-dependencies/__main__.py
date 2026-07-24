import pulumi
import pulumi_simple as simple
import pulumi_simple_invoke as simple_invoke

first = simple.Resource("first", value=False)
# assert that resource second depends on resource first
# because it uses .secret from the invoke which depends on first
second = simple.Resource("second", value=simple_invoke.secret_invoke_output(value="hello",
    secret_response=first.value).apply(lambda invoke: invoke.secret))
third = simple_invoke.StringResource("third", text="third")
# third.text is known during preview, but third does not exist yet. SDKs must
# infer the dependency on third from the invoke's arguments and skip the
# invoke while third's ID is unknown: getText fails if it is called before
# third has been created.
data = simple_invoke.get_text_output(text=third.text)
fourth = simple_invoke.StringResource("fourth", text=data.result)
