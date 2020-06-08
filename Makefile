build:
	go get github.com/elastic/elastic-package/cmd/elastic-package

format:
	gofmt -s -w .

vendor:
	go mod tidy
	go mod vendor

check-git-clean:
	git diff-index --quiet HEAD

check: build format vendor check-git-clean
