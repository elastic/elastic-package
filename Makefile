build:
	go get -ldflags "-X github.com/elastic/elastic-package/internal/version.CommitHash=`git describe --always --long --dirty` -X github.com/elastic/elastic-package/internal/version.BuildTime=`date +%s`" \
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
