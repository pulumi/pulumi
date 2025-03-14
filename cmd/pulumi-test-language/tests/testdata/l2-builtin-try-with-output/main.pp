resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

# When accessing an output of a component inside a direct call to try we should have to use apply.
componentTried = try(component1.ref, "fallback").value
output "tryWithOutput" {
  value = componentTried
}

componentTriedNested = try(component1.ref.value, "fallback")
output "tryWithOutputNested" {
  value = componentTriedNested
}

# Invokes produces outputs.  This output will have apply called on it and try
# utilized within the apply.  The result of this apply is already an output
# which has apply called on it again to pull out `result`
resultContainingOutput = try(invoke("simple-invoke:index:myInvoke", {value="hello"}), "fakefallback").result
output "hello" {
  value = resultContainingOutput
}

str = "str"

# This is a regular "try" which will not result in an output producing try.
# Both versions of try which return outputs and non outputs may be produced
# depending on language implementation.
output "tryScalar" {
  value = try(str, "fallback")
}

