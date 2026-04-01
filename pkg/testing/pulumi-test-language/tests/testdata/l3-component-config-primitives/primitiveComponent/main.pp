config boolean bool {}
config float number {}
config integer int {}
config string string {}

resource "res" "primitive:index:Resource" {
  boolean = boolean
  float = float
  integer = integer
  string = string
  numberArray = [-1.0, 0.0, 1.0]
  booleanMap = {
    t = true
    f = false
  }
}
