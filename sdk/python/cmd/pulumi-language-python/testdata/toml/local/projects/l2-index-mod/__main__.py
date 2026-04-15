import pulumi
import pulumi_index_mod as index_mod

res1 = index_mod.indexmine.Resource("res1", text=index_mod.indexmine.concat_world_output(value="hello").apply(lambda invoke: invoke.result))
pulumi.export("out1", res1.call(input="x").apply(lambda call: call.output))
res2 = index_mod.indexmine.nested.Resource("res2", text=index_mod.indexmine.nested.concat_world_output(value="goodbye").apply(lambda invoke: invoke.result))
pulumi.export("out2", res2.call(input="xx").apply(lambda call: call.output))
