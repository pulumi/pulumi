import pulumi
import pulumi_simple_invoke as simple_invoke

first = simple_invoke.StringResource("first", text="first")
second = simple_invoke.StringResource("second", text="second")
# getText fails unless a StringResource has already been created, so an SDK
# that drops the dependsOn option calls it during preview and fails the test.
gated = simple_invoke.get_text_output(text="Goodbye", opts=pulumi.InvokeOutputOptions(depends_on=[first]))
# myInvoke fails when called with an unknown argument, so an SDK that does not
# await the gated invoke before chaining calls it during preview and fails the
# test.
chained = simple_invoke.my_invoke_output(value=gated.result, opts=pulumi.InvokeOutputOptions(depends_on=[second]))
pulumi.export("result", chained.result)
