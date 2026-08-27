output "expandedMax" {
  value = max([1, 2, 3]...)
}

output "expandedMaxWithPrefix" {
  value = max(0, [1, 2, 3]...)
}
