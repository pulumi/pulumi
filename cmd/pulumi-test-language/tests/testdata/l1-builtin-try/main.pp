config "aMap" "map(string)" {}

output "plainTrySuccess" {
  value = try(aMap["a"], "fallback")
}

output "plainTryFailure" {
  value = try(aMap["b"], "fallback")
}

aSecretMap = secret(aMap)

output "outputTrySuccess" {
  value = try(aSecretMap["a"], "fallback")
}

output "outputTryFailure" {
  value = try(aSecretMap["b"], "fallback")
}

# A dynamically typed value, whose field accesses will not be type errors (since the type is not known to the type
# checker), but may fail dynamically, and can thus be used as test inputs to try.
config "anObject" {}

output "dynamicTrySuccess" {
  value = try(anObject.a, "fallback")
}

output "dynamicTryFailure" {
  value = try(anObject.b, "fallback")
}

aSecretObject = secret(anObject)

output "outputDynamicTrySuccess" {
  value = try(aSecretObject.a, "fallback")
}

output "outputDynamicTryFailure" {
  value = try(aSecretObject.b, "fallback")
}

# Check that explicit null values can be returned.
# It's not safe to return a null value directly (see l1-output-null) so wrap these in a list.
output "plainTryNull" {
  value = [try(anObject.opt, "fallback")]
}

output "outputTryNull" {
  value = [try(aSecretObject.opt, "fallback")]
}