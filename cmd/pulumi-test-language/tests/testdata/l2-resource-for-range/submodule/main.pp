config "submoduleListVar" "list(string)" {}
config "submoduleFilterCond" "bool" {}
config "submoduleFilterVariable" "int" {}

resource "submoduleRes" "simple:index:Resource" {
    options {
      range = { for k, v in submoduleListVar : k => v if submoduleFilterCond }
    }

    value = true
}

resource "submoduleResWithApplyFilter" "simple:index:Resource" {
    options {
      range = { for k, v in submoduleListVar : k => v if (submoduleFilterVariable == 1) }
    }

    value = true
}
