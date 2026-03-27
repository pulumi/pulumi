job "build" {
  expr = "done"
}

workflow "main" {
  job "build" {
    uses = "build"
  }
}
