// Fences this program out of the pkg module. It is not built from here: the smoke
// script (run-pod-publish-refs.sh) copies main.go into a scratch dir and writes a
// go.mod there with replace directives pointing at the freshly generated SDK (whose
// path is only known at run time) and the in-repo pulumi SDK.
module oci-refs-consumer

go 1.25
