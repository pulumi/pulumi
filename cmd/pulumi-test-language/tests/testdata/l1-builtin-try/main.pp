str = "str"

# A dynamically typed value, whose field accesses will not be type errors (since the type is not known to the type
# checker), but may fail dynamically, and can thus be used as test inputs to try.
config "object" {}

# This should return "str".
output "trySucceed" {
  value = try(str, object.a, "fallback")
}

# This should return "fallback", since object.a is undefined.
output "tryFallback1" {
  value = try(object.a, "fallback")
}

# This should return "fallback", since neither object.a nor object.b are defined.
output "tryFallback2" {
  value = try(object.a, object.b, "fallback")
}

# This should return 42, since object.a and object.b are undefined.
output "tryMultipleTypes" {
  value = try(object.a, object.b, 42, "fallback")
}
