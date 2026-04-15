import pulumi
import pulumi_module_format as module_format

# This tests that PCL allows both fully specified type tokens, and tokens that only specify the module and
# member name.
# First use the fully specified token to invoke and create a resource.
res1 = module_format.mod.Resource("res1", text=module_format.mod.concat_world_output(value="hello").apply(lambda invoke: invoke.result))
pulumi.export("out1", res1.call(input="x").apply(lambda call: call.output))
# Next use just the module name as defined by the module format
res2 = module_format.mod.Resource("res2", text=module_format.mod.concat_world_output(value="goodbye").apply(lambda invoke: invoke.result))
pulumi.export("out2", res2.call(input="xx").apply(lambda call: call.output))
# First use the fully specified token to invoke and create a resource.
res3 = module_format.mod.nested.Resource("res3", text=module_format.mod.nested.concat_world_output(value="hello").apply(lambda invoke: invoke.result))
pulumi.export("out3", res3.call(input="x").apply(lambda call: call.output))
# Next use just the module name as defined by the module format
res4 = module_format.mod.nested.Resource("res4", text=module_format.mod.nested.concat_world_output(value="goodbye").apply(lambda invoke: invoke.result))
pulumi.export("out4", res4.call(input="xx").apply(lambda call: call.output))
# First use the fully specified token to invoke and create a resource in the index module.
res5 = module_format.Resource("res5", text=module_format.concat_world_output(value="bonjour").apply(lambda invoke: invoke.result))
pulumi.export("out5", res5.call(input="x").apply(lambda call: call.output))
# Next use just the module name as defined by the module format
res6 = module_format.Resource("res6", text=module_format.concat_world_output(value="youkoso").apply(lambda invoke: invoke.result))
pulumi.export("out6", res6.call(input="xx").apply(lambda call: call.output))
# Next use the short, 2 component, form because this is the index module
res7 = module_format.Resource("res7", text=module_format.concat_world_output(value="guten tag").apply(lambda invoke: invoke.result))
pulumi.export("out7", res7.call(input="xxx").apply(lambda call: call.output))
