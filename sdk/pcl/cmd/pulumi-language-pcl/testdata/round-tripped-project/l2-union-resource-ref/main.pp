resource "github" "unionref:index:Github" {}

resource "gitlab" "unionref:index:Gitlab" {}

resource "pipeline" "unionref:index:Pipeline" {
    source = github
    program = { source = gitlab }
    sources = [github, gitlab]
}
