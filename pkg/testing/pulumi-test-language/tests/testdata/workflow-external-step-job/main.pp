step "invert" {
  input_type = {
    input = bool
  }
  expr       = "!input"
}

job "build" {
  input_type = {
    input = bool
  }
  expr       = "invert"

  step "invert" {
    uses = "invert"
  }
}
