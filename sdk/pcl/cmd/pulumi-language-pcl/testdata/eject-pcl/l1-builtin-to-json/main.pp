config "aString" "string" {}
config "aNumber" "number" {}
config "aList" "list(string)" {}
config "aSecret" "string" {
  secret = true
}

output "stringOutput" {
  value = toJSON(aString)
}

output "numberOutput" {
  value = toJSON(aNumber)
}

output "boolOutput" {
  value = toJSON(true)
}

output "arrayOutput" {
  value = toJSON(["x", "y", "z"])
}

output "objectOutput" {
  value = toJSON({
    "key": "value",
    "count": 1
  })
}

# Nested object using config values
nestedObject = {
  "anObject": { 
    "name": aString,
    "items": aList,
  },
  "a_secret": aSecret,
}

output "nestedOutput" {
  value = toJSON(nestedObject)
}
