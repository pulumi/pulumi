config "aMap" "map(string)" {}

output "plainCanSuccess" {
  value = can(aMap["a"])
}

output "plainCanFailure" {
  value = can(aMap["b"])
}

aSecretMap = secret(aMap)

output "outputCanSuccess" {
  value = can(aSecretMap["a"])
}

output "outputCanFailure" {
  value = can(aSecretMap["b"])
}

# A dynamically typed value, whose field accesses will not be type errors (since the type is not known to the type
# checker), but may fail dynamically, and can thus be used as test inputs to can.
config "anObject" {}

output "dynamicCanSuccess" {
  value = can(anObject.a)
}

output "dynamicCanFailure" {
  value = can(anObject.b)
}

aSecretObject = secret(anObject)

output "outputDynamicCanSuccess" {
  value = can(aSecretObject.a)
}

output "outputDynamicCanFailure" {
  value = can(aSecretObject.b)
}
