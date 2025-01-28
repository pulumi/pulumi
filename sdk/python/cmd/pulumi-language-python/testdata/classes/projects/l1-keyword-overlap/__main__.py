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
pulumi.export("output_class", class_)
pulumi.export("output_export", export)
pulumi.export("output_import", import_)
pulumi.export("output_mod", mod)
pulumi.export("output_object", object)
pulumi.export("output_self", self)
pulumi.export("output_this", this)
pulumi.export("output_if", if_)
