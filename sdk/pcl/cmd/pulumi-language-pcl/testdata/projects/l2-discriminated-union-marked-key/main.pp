resource "first" "discriminated-union-marked-key:index:Example" {
  unionIn = { discriminantKind = "variant2", field2 = "known" }
}

resource "second" "discriminated-union-marked-key:index:Example" {
  unionIn = first.unionOut
}

output "out" {
  value = second.unionOut
}
