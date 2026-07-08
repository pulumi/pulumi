# A resource whose computed output feeds the invoke, forcing the invoke into its
# output-versioned form so that `values` is an Output.
resource "source" "nestedobject:index:Container" {
  inputs = ["alpha", "bravo", "charlie"]
}

values = invoke("nestedobject:index:getValues", {
  names = source.inputs
})

# Ranges over the length of the invoke's computed list and indexes that same
# Output-typed list by the loop counter inside the body. This is the shape from
# https://github.com/pulumi/pulumi/issues/12507.
resource "routes" "nestedobject:index:Target" {
  options {
    range = length(values.results)
  }
  name = values.results[range.value]
}
