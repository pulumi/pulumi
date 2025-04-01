# Make this a component/submodule so that parent references are generated in TS
component "test" "./submodule" {
  name = "fakename"
}

output "foo" {
  value = test.someOutput
}
