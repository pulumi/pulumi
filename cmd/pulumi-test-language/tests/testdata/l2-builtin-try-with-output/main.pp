resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

output "tryWithOutput" {
  value = try(component1.ref, "failure")
}

str = "str"

# Include one more "regular" non nested output try. In this case, 2 try
# functions may be generated in the tested language, one for returning outputs
# and the other for a regular value "any"
output "tryScalar" {
  value = try(str, "fallback")
}

