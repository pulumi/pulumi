import pulumi
import pulumi_simple_invoke as simple_invoke

pulumi.export("result", simple_invoke.invoke_with_default_output().result)
pulumi.export("explicitResult", simple_invoke.invoke_with_default_output(value="explicit").result)
