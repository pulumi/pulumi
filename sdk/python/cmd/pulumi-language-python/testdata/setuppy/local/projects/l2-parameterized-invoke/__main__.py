import pulumi
import pulumi_subpackage as subpackage

pulumi.export("parameterValue", subpackage.do_hello_world_output(input="goodbye").apply(lambda invoke: invoke.output))
