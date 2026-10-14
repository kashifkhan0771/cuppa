.PHONY: build test lint fmt vet check check-data build-data

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

# same checks CI runs, in the same order
check: vet lint test

# validate the question files in questions/
check-data:
	go run ./internal/tools/datagen check

# validate, then write dist/questions.json and dist/categories/
build-data:
	go run ./internal/tools/datagen build
