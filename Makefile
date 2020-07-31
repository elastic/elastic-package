build:
	go get github.com/elastic/elastic-package

format:
	gofmt -s -w .

lint:
    go get golang.org/x/lint/golint
	go list ./... | xargs -n 1 golint -set_exit_status

gomod:
	go mod tidy

check-git-clean:
	git diff-index --quiet HEAD

check: build format lint gomod check-git-clean
