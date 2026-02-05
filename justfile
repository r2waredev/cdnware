# Build the cdnware binary
build:
    go build

# Run tests
test:
    go test -v

# Run tests with coverage
test-cover:
    go test -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html
