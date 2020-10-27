.PHONY: build

build:
	go get -ldflags "-X github.com/elastic/elastic-package/internal/version.CommitHash=`git describe --always --long --dirty` -X github.com/elastic/elastic-package/internal/version.BuildTime=`date +%s`" \
	    github.com/elastic/elastic-package

clean:
	rm -rf build

format:
	go get -u golang.org/x/tools/cmd/goimports
	goimports -local github.com/elastic/elastic-package/ -w .

lint:
	go get -u golang.org/x/lint/golint
	go list ./... | xargs -n 1 golint -set_exit_status

licenser:
	go get -u github.com/elastic/go-licenser
	go-licenser -license Elastic

gomod:
	go mod tidy

test-go:
	# -count=1 is included to invalidate the test cache. This way, if you run "make test-go" multiple times
	# you will get fresh test results each time. For instance, changing the source of mocked packages
	# does not invalidate the cache so having the -count=1 to invalidate the test cache is useful.
	go test -v -count 1 ./...

test-stack-command:
	./scripts/test-stack-command.sh

test-check-packages:
	./scripts/test-check-packages.sh

test: test-go test-stack-command test-check-packages

check-git-clean:
	git update-index --really-refresh
	git diff-index --quiet HEAD

check: build format lint licenser gomod test check-git-clean
