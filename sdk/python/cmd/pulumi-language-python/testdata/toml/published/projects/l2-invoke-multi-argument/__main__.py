import pulumi
import pulumi_multi_argument_invoke as multi_argument_invoke

pulumi.export("both", multi_argument_invoke.multi_argument_invoke_output("hello", "world").apply(lambda invoke: invoke.result))
pulumi.export("onlyRequired", multi_argument_invoke.multi_argument_invoke_output("hello").apply(lambda invoke: invoke.result))
