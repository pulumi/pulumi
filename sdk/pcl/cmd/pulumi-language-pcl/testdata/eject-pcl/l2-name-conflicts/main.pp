config "names" "bool" {
  default = true
}

config "Names" "bool" {
  default = true
}

config "mod" "string" {
  default = "module"
}

config "Mod" "string" {
  default = "format"
}

resource "namesResource" "names:mod:Res" {
  value = names
}

resource "modResource" "module-format:mod:Resource" {
  text = "${mod}-${Mod}"
}

output "namesResourceVal" {
  value = namesResource.value
}

output "modResourceText" {
  value = modResource.text
}

output "nameVariables" {
  value = names && Names
}

output "modVariables" {
  value = "${mod}-${Mod}"
}
