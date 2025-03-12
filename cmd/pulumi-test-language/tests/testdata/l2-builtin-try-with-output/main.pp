resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

output "tryWithOutput" {
  value = try(component1.ref, "failure")
}

# This should result in a try who's result is an output. 
# It seems to generate the correct function, likely due to checking independently when generating 
# the call site, but when generating it is treated as a regular value, and not an output that would require .apply.
# The generated code is marked with this comment.
# Ideas? Debugging Tips?
resultContainingOutput = try(invoke("simple-invoke:index:myInvoke", {value="hello"}))
output "hello" {
  value = resultContainingOutput.result
}

# This generates fine but without a null check, so perhaps it is actually a non output after all?
resultContaingOutputWithoutTry = invoke("simple-invoke:index:myInvoke", {value="hello"})
output "helloNoTry" {
  value = resultContaingOutputWithoutTry.result
}

str = "str"

# Include one more "regular" non nested output try. In this case, 2 try
# functions may be generated in the tested language, one for returning outputs
# and the other for a regular value "any"
output "tryScalar" {
  value = try(str, "fallback")
}

