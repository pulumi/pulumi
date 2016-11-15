PROJECT=github.com/marapongo/mu

all:
	go install ${PROJECT}
	golint ${PROJECT}
	go vet ${PROJECT}

