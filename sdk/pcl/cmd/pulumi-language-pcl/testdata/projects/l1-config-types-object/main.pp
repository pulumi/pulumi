config "aMap" "map(int)" {}

output "theMap" {
  value = {
    a: aMap["a"] + 1,
    b: aMap["b"] + 1,
  }
}

config "anObject" "object({prop=list(bool)})" {}

output "theObject" {
  value = anObject.prop[0]
}

config "anyObject" {}

output "theThing" {
  value = anyObject.a + anyObject.b
}

config "optionalUntypedObject" {
  default = { "key" = "value" }
}

output "defaultUntypedObject" {
  value = optionalUntypedObject
}

config "optionalList" "list(string)" {
  default = null
}

config "optionalMap" "map(string)" {
  default = null
}

config "optionalObject" "object({prop=string, other=int})" {
  default = null
}

output "optionalList" {
  value = optionalList == null ? "null" : toJSON(optionalList)
}

output "optionalMap" {
  value = optionalMap == null ? "null" : toJSON(optionalMap)
}

output "optionalObject" {
  value = optionalObject == null ? "null" : toJSON(optionalObject)
}
