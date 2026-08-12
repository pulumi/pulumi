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

# An untyped (dynamic) config value. Pins iterating dynamic entries in generated programs
# (e.g. TypeScript's Object.entries over a value with no static type).
config "alternativeNames" {
  default = {}
}

output "names" {
  value = [for entry in entries(alternativeNames) : entry.value]
}
