config value "string" {}

config tags "map(string)" {
    default = {
        "interpolated/${value}" = "value"
    }
}