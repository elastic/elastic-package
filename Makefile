build:
	go get -ldflags "-X github.com/elastic/elastic-package/cmd.CommitHash=`git describe --always --long --dirty` -X github.com/elastic/elastic-package/cmd.BuildTime=`date +%FT%T%z`" \
	    github.com/elastic/elastic-package

format:
	gofmt -s -w .

lint:
	go get -u golang.org/x/lint/golint
	go list ./... | xargs -n 1 golint -set_exit_status

gomod:
	go mod tidy

check-git-clean:
	git update-index --really-refresh
	git diff-index --quiet HEAD

check: build format lint gomod check-git-clean
