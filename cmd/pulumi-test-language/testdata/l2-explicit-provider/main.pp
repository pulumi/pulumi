resource "prov" "pulumi:providers:simple" {

}

resource "res" "simple:index:Resource" {
    options {
        provider = prov
    }
    
    value = true
}