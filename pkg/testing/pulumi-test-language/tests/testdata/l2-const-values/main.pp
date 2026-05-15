resource "instance" "const-values:index:Resource" {
  name = "example"
  nested = {
    value = "inner"
  }
  arrayItems = [
    { value = "first" },
    { value = "second" },
  ]
  mapItems = {
    one = { value = "one-value" }
    two = { value = "two-value" }
  }
}

output "invokeResult" {
  value = invoke("const-values:index:applyConst", {
    name = "example"
    nested = {
      value = "inner"
    }
    arrayItems = [
      { value = "first" },
      { value = "second" },
    ]
    mapItems = {
      one = { value = "one-value" }
      two = { value = "two-value" }
    }
  })
}
