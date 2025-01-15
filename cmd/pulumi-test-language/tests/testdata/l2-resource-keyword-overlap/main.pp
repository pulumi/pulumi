resource "class" "simple:index:Resource" {
  value = true
}

output "class" {
  value = class
}

resource "export" "simple:index:Resource" {
  value = true
}

output "export" {
  value = export
}

resource "mod" "simple:index:Resource" {
  value = true
}

output "mod" {
  value = mod
}

resource "import" "simple:index:Resource" {
  value = true
}

# TODO(pulumi/pulumi#18246): Pcl should support scoping based on resource type just like HCL does in TF so we can uncomment this.
# output "import" {
#   value = Resource["import"]
# }

resource "object" "simple:index:Resource" {
  value = true
}

output "object" {
  value = object
}

resource "self" "simple:index:Resource" {
  value = true
}

output "self" {
  value = self
}

resource "this" "simple:index:Resource" {
  value = true
}

output "this" {
  value = this
}
