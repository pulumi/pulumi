resource cluster "conflicta:mod:Cluster" {}
resource nginx "conflictb:mod:Service" {
    cluster = cluster.arn
}
