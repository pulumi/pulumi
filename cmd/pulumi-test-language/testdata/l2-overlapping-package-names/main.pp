resource "one" "simple:overlapping_pkg:Resource" {
    __logicalName = "one"
    value = true
}

resource "two" "simpleoverlap:overlapping_pkg:OverlapResource" {
    __logicalName = "two"
    value = true
}

output "outOne" {
  __logicalName = "outOne"
  value = one
}

output "outTwo" {
  __logicalName = "outTwo"
  value = two.out
}
