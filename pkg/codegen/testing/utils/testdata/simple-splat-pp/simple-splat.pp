allKeys = invoke("splat:index:getSshKeys", {})

resource "main" "splat:index:Server" {
  sshKeys = allKeys.sshKeys[*].name
}
