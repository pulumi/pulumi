// Test that the ID type is treated the same as a string type, despite being tracked as a distinct type. This 
// includes directly passing it to string fields, but also for bool and numeric values being able to cast to it.

resource "source1" "primitive:index:Resource" {
  boolean = false
  float = 1
  integer = 2
  string = "1234"
  numberArray = [3]
  booleanMap = {
    source = false
  }
}

resource "source2" "primitive:index:Resource" {
  boolean = false
  float = 1
  integer = 2
  string = "true"
  numberArray = [3]
  booleanMap = {
    source = false
  }
}

idMap = {
  source1Token = source1.id
  source2Token = source2.id
}

resource "sink1" "primitive:index:Resource" {
  boolean = false
  float = idMap["source1Token"]
  integer = idMap["source1Token"]
  string = idMap["source1Token"]
  numberArray = [idMap["source1Token"]]
  booleanMap = {
    sink = false
  }
}

resource "sink2" "primitive:index:Resource" {
  boolean = idMap["source2Token"]
  float = 1
  integer = 2
  string = "abc"
  numberArray = [3]
  booleanMap = {
    sink = idMap["source2Token"]
  }
}

output "ids" {
  value = idMap
}

// test an id value can flow through a string function
output "base64" { 
  value = toBase64(sink2.id)
}
