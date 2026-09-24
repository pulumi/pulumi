config "from" "int" {}
config "to" "int" {}

output "rangeTo" {
  value = range(to)
}

output "rangeFromTo" {
  value = range(from, to)
}

output "literalTo" {
  value = range(3)
}

output "literalFromTo" {
  value = range(2, 5)
}

output "literalEmpty" {
  value = range(3, 3)
}
