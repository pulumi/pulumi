#!/usr/bin/env bash
set -o errexit

export PKGS='./pkg/... ./cmd/...'
export PKGS_COMMA='./pkg/...,./cmd/...'

go test -i ${PKGS}
go list -f '{{if gt (len .TestGoFiles) 0}}"go test -covermode count -coverprofile {{.Name}}.coverprofile -coverpkg $PKGS_COMMA {{.ImportPath}}"{{end}}' $PKGS | xargs -P100 -I {} bash -c {} 2>&1 | grep -v '^warning: no packages being tested depend on '
gocovmerge $(ls *.coverprofile) > coverage.cov
go tool cover -func=coverage.cov
rm *.coverprofile

exit 0