import pulumi

# Keywords in various languages should be renamed and work.
class_ = "class_output_string"
export = "export_output_string"
import_ = "import_output_string"
mod = "mod_output_string"
object = {
    "object": "object_output_string",
}
self = "self_output_string"
this = "this_output_string"
if_ = "if_output_string"
pulumi.export("class", class_)
pulumi.export("export", export)
pulumi.export("import", import_)
pulumi.export("mod", mod)
pulumi.export("object", object)
pulumi.export("self", self)
pulumi.export("this", this)
pulumi.export("if", if_)
