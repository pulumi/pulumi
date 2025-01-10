# Since the name is "this" it will fail in typescript and other languages with
# this reservered keyword if it is not renamed.
this = "somestring"
cls = "class"
export = "export"
import = "import"
object = { object = "object" }
module = "module"
self = "self"
this = "this"

output "output_this" {
  value = this
}

output "output_cls" {
  value = cls
}

output "output_export" {
  value = export
}

output "output_import" {
  value = import
}

output "output_object" {
  value = object
}

output "output_module" {
  value = module
}

output "output_self" {
  value = self
}
