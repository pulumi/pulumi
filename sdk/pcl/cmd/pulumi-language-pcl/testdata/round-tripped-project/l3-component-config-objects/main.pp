config "plainNumberArray" "list(number)" {}
config "plainBooleanMap" "map(bool)" {}
config "secretNumberArray" "list(number)" {
  secret = true
}
config "secretBooleanMap" "map(bool)" {
  secret = true
}

component "plain" "./primitiveComponent" {
  numberArray = plainNumberArray
  booleanMap = plainBooleanMap
}

component "secret" "./primitiveComponent" {
  numberArray = secretNumberArray
  booleanMap = secretBooleanMap
}
