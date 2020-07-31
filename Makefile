.PHONY: vendor


build:
	go get github.com/elastic/elastic-package

format:
	go fmt $(go list ./... | grep -v /vendor/)

lint:
	GO111MODULE=off go get -u golang.org/x/lint/golint
	go list ./... | grep -v /vendor/ | xargs -n 1 golint -set_exit_status

vendor:
	go mod tidy
	go mod vendor

check-git-clean:
	git diff-index --quiet HEAD && echo ok

check: build format lint vendor check-git-clean
