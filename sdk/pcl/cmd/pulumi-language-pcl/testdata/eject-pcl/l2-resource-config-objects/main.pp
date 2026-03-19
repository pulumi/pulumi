config "plainNumberArray" "list(number)" {}
config "plainBooleanMap" "map(bool)" {}
config "secretNumberArray" "list(number)" {
  secret = true
}
config "secretBooleanMap" "map(bool)" {
  secret = true
}

resource "plain" "primitive:index:Resource" {
  boolean = true
  float = 3.5
  integer = 3
  string = "plain"
  numberArray = plainNumberArray
  booleanMap = plainBooleanMap
}

resource "secret" "primitive:index:Resource" {
  boolean = true
  float = 3.5
  integer = 3
  string = "secret"
  numberArray = secretNumberArray
  booleanMap = secretBooleanMap
}
