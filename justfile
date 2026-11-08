set dotenv-load
set positional-arguments

bin := "bin/prwatch"

# Show all open PRs/MRs (same as `just list`)
default: list

# List open PRs/MRs. Flags: --all (default) --github --gitlab --keep --urls --width N
list *args: build
    @./{{bin}} list "$@"

# Shortcut for `just list --github`
github *args: (list "--github" args)

# Shortcut for `just list --gitlab`
gitlab *args: (list "--gitlab" args)

# Peek without marking new activity as seen
peek *args: (list "--keep" args)

# Forget what was seen; the next run becomes the new baseline
reset: build
    @./{{bin}} reset

build:
    @go build -o {{bin}} .

test:
    go test ./...

fmt:
    gofmt -w .
    go vet ./...

# Install prwatch into $GOBIN / ~/go/bin
install:
    go install .

clean:
    rm -rf bin
