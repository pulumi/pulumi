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

output "lookupLiteral" {
  value = lookup({"a": 1, "b": 2}, "c", 3)
}

output "lookupLiteralMismatchedType" {
  value = lookup({"a": 1, "b": 2}, "c", true)
}

output "deprecatedTwoArgumentLookup" {
  value = lookup({"a": 1, "b": 2}, "a")
}

# An untyped (dynamic) config value. Pins iterating dynamic entries in generated programs
# (e.g. TypeScript's Object.entries over a value with no static type).
config "alternativeNames" {
  default = {}
}

output "names" {
  value = [for entry in entries(alternativeNames) : entry.value]
}

output "lengthOutput" {
  value = length(aMap)
}

output "lengthDynamicOutput" {
  value = length(alternativeNames)
}
