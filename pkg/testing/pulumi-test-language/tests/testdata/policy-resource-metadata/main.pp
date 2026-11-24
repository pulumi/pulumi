resource "prov" "pulumi:providers:simple" {
}

resource "identity" "simple:index:Resource" {
    value = true
}

resource "protected" "simple:index:Resource" {
    value = true
    options {
        protect = true
    }
}

resource "ignoreChanges" "simple:index:Resource" {
    value = true
    options {
        ignoreChanges = [value]
    }
}

resource "deleteBeforeReplace" "simple:index:Resource" {
    value = true
    options {
        deleteBeforeReplace = true
    }
}

resource "secretOutput" "simple:index:Resource" {
    value = true
    options {
        additionalSecretOutputs = [value]
    }
}

resource "customTimeouts" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            create = "5m"
        }
    }
}

resource "explicitProvider" "simple:index:Resource" {
    value = true
    options {
        provider = prov
    }
}

resource "parent" "simple:index:Resource" {
    value = true
}

resource "child" "simple:index:Resource" {
    value = true
    options {
        parent = parent
    }
}

resource "dependency" "simple:index:Resource" {
    value = true
}

resource "dependsOn" "simple:index:Resource" {
    value = true
    options {
        dependsOn = [dependency]
    }
}

resource "propertyDependency" "simple:index:Resource" {
    value = dependency.value
}
