config "names" "list(optional(string))" {
  default = [null, "hello", null]
}

output "namesLength" {
  value = length(names)
}
