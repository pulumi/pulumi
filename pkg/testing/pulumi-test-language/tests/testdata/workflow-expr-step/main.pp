step "invert" {
  input_type = "bool"
  expr       = "!input"
}

workflow "main" {
  job "build" {
    step "invert" {
      uses = "invert"
    }
  }
}
