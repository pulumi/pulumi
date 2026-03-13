config "aMap" "map(string)" {}

output "entriesOutput" {
  value = entries(aMap)
}

output "lookupOutput" {
  value = lookup(aMap, "keyPresent", "default")
}

output "lookupOutputDefault" {
  value = lookup(aMap, "keyMissing", "default")
}
