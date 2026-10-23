resource "source" "nestedobject:index:Container" {
  inputs = ["a", "b"]
}

resource "sink" "nestedobject:index:Container" {
  inputs = source.details[*].value
}

resource "setOptional" "optional-primitive-ref:index:Resource" {
  data = {}
  optionalData = {
    string = "set"
  }
}

resource "unsetOptional" "optional-primitive-ref:index:Resource" {
  data = {}
}

output "setOptionalSplat" {
  value = setOptional.optionalData[*].string
}

output "unsetOptionalSplat" {
  value = unsetOptional.optionalData[*].string
}
