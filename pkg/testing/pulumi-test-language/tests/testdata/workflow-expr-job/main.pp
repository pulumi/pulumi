step "invert" {
  input_type = "bool"
  expr       = "!input"
}

job "build" {
  expr = "invert"

  step "invert" {
    uses = "invert"
  }
}

workflow "main" {
  job "build" {
    uses = "build"
  }
}
