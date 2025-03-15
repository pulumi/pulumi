resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

output "canWithOutput" {
  value = can(component1.ref)
}

str = "str"

# Include one more "regular" non nested output can. In this case, 2 can
# functions may be generated in the tested language, one for returning pulumi output of type bool
# and the other for a regular bool
output "canScalar" {
  value = can(str)
}

