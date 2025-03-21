config "aNumber" "number" {}

output "theNumber" {
  value = aNumber + 1.25
}

config "aString" "string" {}

output "theString" {
  value = aString + " World"
}

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
