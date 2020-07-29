.PHONY: vendor


build:
	go get github.com/elastic/elastic-package/cmd/elastic-package

format:
	gofmt -s -w .

lint:
	GO111MODULE=off go get -u golang.org/x/lint/golint
	go list ./... | grep -v /vendor/ | xargs -n 1 golint -set_exit_status

vendor:
	go mod tidy
	go mod vendor

check-git-clean:
	git diff-index --quiet HEAD

check: build format lint vendor check-git-clean
