step "constant" {
  expr = "done"
}

workflow "main" {
  job "build" {
    step "constant" {
      uses = "constant"
    }
  }
}
