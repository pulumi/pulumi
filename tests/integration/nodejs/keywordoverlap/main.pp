# Since the name is "this" it will fail in typescript and other languages with
# this reservered keyword if it is not renamed.
this = "somestring"

output "output" {
  value = this
}
