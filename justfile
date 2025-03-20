# Help
help:
  just -l

# Build
build:
  go build

# Install locally
install:
  go install

# Run code audit checks
check:
  go tool staticcheck ./...
  go tool govulncheck ./...
  go tool errcheck ./...
