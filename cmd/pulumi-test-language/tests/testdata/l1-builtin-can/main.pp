str = "str"

# A dynamically typed value, whose field accesses will not be type errors (since the type is not known to the type
# checker), but may fail dynamically, and can thus be used as test inputs to can.
config "object" {}

anotherObject = {
  nested = "nestedValue"
}

# This should return false, since object.a is undefined.
output "canFalse" {
  value = can(object.a)
}

output "canFalseDoubleNested" {
  value = can(object.a.b)
}

# This should return true, since anotherObject.nested is defined.
output "canTrue" {
  value = can(anotherObject.nested)
}
