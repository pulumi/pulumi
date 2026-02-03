config "aMap" "map(string)" {}

output "plainTrySuccess" {
  value = can(aMap["a"])
}

output "plainTryFailure" {
  value = can(aMap["b"])
}

aSecretMap = secret(aMap)

output "outputTrySuccess" {
  value = can(aSecretMap["a"])
}

output "outputTryFailure" {
  value = can(aSecretMap["b"])
}

# A dynamically typed value, whose field accesses will not be type errors (since the type is not known to the type
# checker), but may fail dynamically, and can thus be used as test inputs to can.
config "anObject" {}

output "dynamicTrySuccess" {
  value = can(anObject.a)
}

output "dynamicTryFailure" {
  value = can(anObject.b)
}

aSecretObject = secret(anObject)

output "outputDynamicTrySuccess" {
  value = can(aSecretObject.a)
}

output "outputDynamicTryFailure" {
  value = can(aSecretObject.b)
}

# Check that explicit null values can be returned
output "plainTryNull" {
  value = can(anObject.opt)
}

output "outputTryNull" {
  value = can(aSecretObject.opt)
}