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

resource "sink4" "enum:index:Deluxe" {
  numberEnum = 0.1
  wordyEnum = "It's got apostrophes"
  arrayOfEnum = ["one", "two"]
  mapOfEnum = {
    small = 1
    large = 2
  }
  holder = {
    size = 2
    color = "one"
  }
  unionEnum = "A Value With Spaces."
}