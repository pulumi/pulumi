resource assert "lambda:lambda:Lambda" {
    lambda = "dns"
}

output global {
    value = assert.lambda
}
