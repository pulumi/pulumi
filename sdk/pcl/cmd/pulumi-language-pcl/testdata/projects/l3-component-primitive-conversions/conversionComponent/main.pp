config boolean bool {}
config float number {}
config integer int {}
config string string {}

resource "res" "primitive:index:Resource" {
  boolean = boolean
  float = float
  integer = integer
  string = string
  numberArray = [2.0, 42.0, 6.5]
  booleanMap = {
    fromBool = true
    fromString = true
  }
}

output boolean {
  value = res.boolean
}

output float {
  value = res.float
}

output integer {
  value = res.integer
}

output string {
  value = res.string
}
