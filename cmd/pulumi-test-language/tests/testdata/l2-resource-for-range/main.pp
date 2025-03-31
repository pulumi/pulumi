config "listVar" "list(string)" {
  default = ["one", "two", "three"]
}

config "filterCond" "bool" {
  default = true
}

resource "res" "simple:index:Resource" {
    options {
      range = { for k, v in listVar : k => v if filterCond }
    }

    value = true
}

eventualListVar = secret(listVar)

resource "eventualRes" "simple:index:Resource" {
    options {
      range = { for k, v in eventualListVar : k => v if filterCond }
    }

    value = true
}

component "submoduleComp" "./submodule" {
  submoduleListVar = ["one"]
  submoduleFilterCond = true
  submoduleFilterVariable = 1
}
