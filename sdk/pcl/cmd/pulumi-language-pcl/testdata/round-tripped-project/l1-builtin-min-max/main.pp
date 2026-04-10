config "a" "number" {}
config "b" "number" {}
config "c" "int" {}
config "d" "int" {}

output "maxResult" {
  value = max(a, b)
}

output "minResult" {
  value = min(a, b)
}

output "intMaxResult" {
  value = max(c, d)
}

output "intMinResult" {
  value = min(c, d)
}
