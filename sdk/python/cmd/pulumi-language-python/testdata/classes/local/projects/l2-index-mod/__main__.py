import pulumi
import pulumi_index_mod as index_mod

res1 = index_mod.indexmine.Resource("res1", text=index_mod.indexmine.concat_world_output(value="hello").result)
pulumi.export("out1", res1.call(input="x").output)
res2 = index_mod.indexmine.nested.Resource("res2", text=index_mod.indexmine.nested.concat_world_output(value="goodbye").result)
pulumi.export("out2", res2.call(input="xx").output)
