import pulumi
import pulumi_simple as simple

class_ = simple.Resource("class", value=True)
pulumi.export("class", class_)
export = simple.Resource("export", value=True)
pulumi.export("export", export)
mod = simple.Resource("mod", value=True)
pulumi.export("mod", mod)
import_ = simple.Resource("import", value=True)
# TODO(pulumi/pulumi#18246): Pcl should support scoping based on resource type just like HCL does in TF so we can uncomment this.
# output "import" {
#   value = Resource["import"]
# }
object = simple.Resource("object", value=True)
pulumi.export("object", object)
self = simple.Resource("self", value=True)
pulumi.export("self", self)
this = simple.Resource("this", value=True)
pulumi.export("this", this)
if_ = simple.Resource("if", value=True)
pulumi.export("if", if_)
