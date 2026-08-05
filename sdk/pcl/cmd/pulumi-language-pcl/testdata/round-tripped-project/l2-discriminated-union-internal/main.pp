resource "example1" "discriminated-union-internal:index:Example" {
  unionOf = { __type = "Alpha", payload = "p1", weight = 1 }
  secretUnion = { __type = "Beta", payload = "s1", tint = "blue" }
}

resource "example2" "discriminated-union-internal:index:Example" {
  unionOf = { __type = "Beta", payload = "p2", tint = "red" }
}

resource "example3" "discriminated-union-internal:index:Example" {
  unionOf = { __type = "Gamma", payload = "p3", active = true }
}
