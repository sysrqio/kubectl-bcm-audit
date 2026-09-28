VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/sysrqio/kubectl-bcm-audit/internal/version.Version=$(VERSION) \
           -X github.com/sysrqio/kubectl-bcm-audit/internal/version.Commit=$(COMMIT) \
           -X github.com/sysrqio/kubectl-bcm-audit/internal/version.Date=$(DATE)

.PHONY: build test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/kubedrift-audit ./cmd/kubedrift-audit
	ln -sf kubedrift-audit bin/kubectl-bcm-audit

test:
	go test ./... -count=1

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"

clean:
	rm -rf bin/
