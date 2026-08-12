resource "provElided" "pulumi:providers:output" {
    elideUnknowns = true
}

resource "provNotElided" "pulumi:providers:output" {}

resource "topLevelElided" "output:index:Resource" {
    value = 1
    options {
        provider = provElided
    }
}

resource "topLevelNotElided" "output:index:Resource" {
    value = 1
    options {
        provider = provNotElided
    }
}

output "topLevelElided" {
    value = topLevelElided.secretOutput
}

output "topLevelNotElided" {
    value = topLevelNotElided.secretOutput
}
