import pulumi
import pulumi_synthetic as synthetic

rt = synthetic.resource_properties.Root("rt")
pulumi.export("trivial", rt)
pulumi.export("simple", rt.res1)
pulumi.export("foo", rt.res1.obj1.res2.obj2)
pulumi.export("complex", rt.res1.obj1.res2.obj2.answer)
