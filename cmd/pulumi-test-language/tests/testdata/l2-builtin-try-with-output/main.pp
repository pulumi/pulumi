resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

# TODO(pulumi/pulumi#18895) When value is directly a scope traversal inside the
# output this fails to generate the "apply" call. eg if the output's internals
# are `value = componentTried.value`
#
# Apply is used when a resource output attribute is accessed.
componentTried = try(component1.ref, "fallback").value
output "tryWithOutput" {
  value = componentTried
}

componentTriedNested = try(component1.ref.value, "fallback")
output "tryWithOutputNested" {
  value = componentTriedNested
}

# Invokes produces outputs. 
# This output will have apply called on it and try utilized within the apply.
# The result of this apply is already an output which has apply called on it
# again to pull out `result`
resultContainingOutput = try(invoke("simple-invoke:index:myInvoke", {value="hello"}), "fakefallback").result
output "hello" {
  value = resultContainingOutput
}

str = "str"

# This is a regular "try" which will not result in an output producing try.
# Both try and tryOutput which return outputs and non outputs respectively may be produced
# depending on language implementation.  They should both be able to coexist.
output "tryScalar" {
  value = try(str, "fallback")
}

