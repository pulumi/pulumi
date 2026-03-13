config "aString" "string" {}
config "aNumber" "number" {}
config "aList" "list(string)" {}
config "aSecret" "string" {
  secret = true
}

# Literal data shapes built as locals
literalBool = true
literalArray = ["x", "y", "z"]
literalObject = {
  "key": "value",
  "count": 1
}

# Nested object using config values
nestedObject = {
  "name": aString,
  "items": aList,
  "a_secret": aSecret,
}

output "stringOutput" {
  value = toJSON(aString)
}

output "numberOutput" {
  value = toJSON(aNumber)
}

output "boolOutput" {
  value = toJSON(literalBool)
}

output "arrayOutput" {
  value = toJSON(literalArray)
}

output "objectOutput" {
  value = toJSON(literalObject)
}

output "nestedOutput" {
  value = toJSON(nestedObject)
}
