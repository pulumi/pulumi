resource "example1" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant1", payload = "p1", extra = "e1" }
}

resource "example2" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant2", payload = "p2", extra = "e2" }
}

resource "example3" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant3", payload = "p3", count = 3 }
}

resource "example4" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant4", payload = "p4", enabled = true }
}

resource "example5" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant5", payload = "p5", label = "l5" }
}

resource "example6" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant6", payload = "p6", code = 6 }
}

resource "example7" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant7", payload = "p7", message = "m7" }
}

resource "example8" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant8", payload = "p8", size = 8 }
}

resource "example9" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant9", payload = "p9", flag = false }
}

resource "example10" "discriminated-union-many:index:Example" {
  unionOf = { discriminantKind = "variant10", payload = "p10", note = "n10" }
}
