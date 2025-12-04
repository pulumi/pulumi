import pulumi
import pulumi_names as names

res1 = names.ResMap("res1", value=True)
res2 = names.ResArray("res2", value=True)
res3 = names.ResList("res3", value=True)
res4 = names.ResResource("res4", value=True)
