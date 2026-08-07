resource "example1" "discriminated-union-internal:index:Example" {
  unionOf = { type__ = "Alpha", payload = "p1", weight = 1 }
  secretUnion = { type__ = "Beta", payload = "s1", tint = "blue" }
}

resource "example2" "discriminated-union-internal:index:Example" {
  unionOf = { type__ = "Beta", payload = "p2", tint = "red" }
}

resource "example3" "discriminated-union-internal:index:Example" {
  unionOf = { type__ = "Gamma", payload = "p3", active = true }
}
