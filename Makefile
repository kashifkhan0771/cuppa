.PHONY: build test lint fmt vet check check-data build-data cover check-dashes

# total test coverage must be above this percent
MIN_COVERAGE := 80

# compile every package; binaries go to bin/
build:
	go build -o bin/ ./...

# run all tests with the race detector on
test:
	go test -race ./...

# run the linters configured in .golangci.yml
lint:
	golangci-lint run

# rewrite Go files into standard format
fmt:
	gofmt -w .

vet:
	go vet ./...

# the main checks CI runs, in the same order
check: vet lint cover check-dashes

# run all tests, then fail if total coverage is not above MIN_COVERAGE
cover:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk -v min=$(MIN_COVERAGE) '/^total:/ { \
		sub("%", "", $$3); \
		print "total coverage: " $$3 "%, must be above " min "%"; \
		if ($$3 + 0 <= min) exit 1 \
	}'

# fail if any file has an en dash or em dash; the bytes are written
# as octal so this Makefile doesn't contain the characters itself
check-dashes:
	@if git grep -nIF --untracked -e "$$(printf '\342\200\223')" -e "$$(printf '\342\200\224')"; then \
		echo "found long dashes above: use a plain hyphen, colon or comma"; \
		exit 1; \
	fi

# validate the question files in questions/
check-data:
	go run ./internal/tools/datagen check

# validate, then write dist/questions.json and dist/categories/
build-data:
	go run ./internal/tools/datagen build
