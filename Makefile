PROJECT=github.com/marapongo/mu
PROJECT_PKGS=$(shell go list ./... | grep -v /vendor/)

all:
	go test ${PROJECT_PKGS}
	go install ${PROJECT}
	golint ${PROJECT}
	go vet ${PROJECT}

