resource "component1" "component:index:ComponentCustomRefOutput" {
  value = "foo-bar-baz"
}

componentCanShouldBeTrue = can(component1.ref)
output "componentCan" {
  value = componentCanShouldBeTrue
}

invokeCanShouldBeTrue = can(invoke("simple-invoke:index:myInvoke", {value="hello"}))
output "invokeCan" {
  value = invokeCanShouldBeTrue
}

ternaryShouldNotUseApply = can(true) ? "option_one" : "option_two"
output "ternaryCan" {
  value = ternaryShouldNotUseApply
}

ternaryShouldUseApply = can(component1.ref) ? "option_one" : "option_two"
output "ternaryCanOutput" {
  value = ternaryShouldUseApply
}

str = "str"
output "scalarCan" {
  value = can(str)
}
