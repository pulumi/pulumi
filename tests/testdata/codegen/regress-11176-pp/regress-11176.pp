resource cluster "aws:ecs/cluster:Cluster" {}
resource nginx "awsx:ecs:FargateService" {
    cluster = cluster.arn
}
