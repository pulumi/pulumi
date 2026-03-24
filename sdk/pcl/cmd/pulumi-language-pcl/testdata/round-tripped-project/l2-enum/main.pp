resource "sink1" "enum:index:Res" {
  intEnum = 1
  stringEnum = "two"
}

resource "sink2" "enum:mod:Res" {
  intEnum = 1
  stringEnum = "two"
}

resource "sink3" "enum:mod/nested:Res" {
  intEnum = 1
  stringEnum = "two"
}