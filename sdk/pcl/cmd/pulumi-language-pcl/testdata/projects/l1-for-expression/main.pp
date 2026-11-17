names = ["alpha", "beta", "gamma"]

tags = {
    "Environment" = "production",
    "Team"        = "infra",
}

output "prefixed" {
    value = [for n in names : "prefix-${n}"]
}

output "filtered" {
    value = [for n in names : n if n != "beta"]
}

output "indexed" {
    value = [for i, n in names : "${i}:${n}"]
}

output "tagList" {
    value = [for k, v in tags : "${k}=${v}"]
}

output "prefixedMap" {
    value = {for n in names : n => "prefix-${n}"}
}

output "filteredTags" {
    value = {for k, v in tags : k => v if k != "Team"}
}
