// example
resource "argocd_serverDeployment" "kubernetes:apps/v1:Deployment" {
  apiVersion = "apps/v1"
  kind = "Deployment"
  metadata = {
    labels = {
      "app.kubernetes.io/component" = "server"
      "aws:region" = "us-west-2"
      "key%percent" = "percent"
      "key...ellipse" = "ellipse"
      "key{bracket" = "bracket"
      "key}bracket" = "bracket"
      "key*asterix" = "asterix"
      "key?question" = "question"
      "key,comma" = "comma"
      "key&&and" = "and"
      "key||or" = "or"
      "key!not" = "not"
      "key=>geq" = "geq"
      "key==eq" = "equal"
    }
    name = "argocd-server"
  }
}
