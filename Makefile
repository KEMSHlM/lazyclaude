BINARY := lazyclaude
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test test-unit test-e2e test-ssh test-visual test-visual-ssh test-vhs lint install clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/lazyclaude

test:
	go test -race -cover ./...

test-unit:
	go test -race -cover ./internal/...

## Docker E2E tests (requires Docker)
test-e2e:
	docker build -f vis_e2e_tests/Dockerfile --target test -t lazyclaude-test .
	docker run --rm lazyclaude-test go test -v -timeout 120s ./tests/

## SSH remote tests (requires Docker Compose)
test-ssh:
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml build
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml run --rm local
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml down

## SSH + real Claude Code E2E
test-ssh-e2e:
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml build
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml run --rm local \
		"ssh-keyscan -H remote >> /root/.ssh/known_hosts 2>/dev/null && bash vis_e2e_tests/verify_remote_popup.sh lazyclaude"
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml down

## Visual E2E tests — capture-pane UI output (requires Docker)
test-visual:
	docker build -f vis_e2e_tests/Dockerfile --target test -t lazyclaude-test .
	docker run --rm lazyclaude-test bash -c '\
		for f in vis_e2e_tests/verify_*.sh; do \
			echo ""; echo "========== $$(basename $$f) =========="; \
			bash "$$f" lazyclaude || exit 1; \
		done'

## Visual E2E: single script (e.g. make test-visual-popup_stack)
test-visual-%:
	docker build -f vis_e2e_tests/Dockerfile --target test -t lazyclaude-test .
	docker run --rm lazyclaude-test bash vis_e2e_tests/verify_$*.sh lazyclaude

## Visual E2E: SSH mode (requires Docker Compose)
test-visual-ssh:
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml build
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml run --rm local \
		"ssh-keyscan -H remote >> /root/.ssh/known_hosts 2>/dev/null && \
		 for f in vis_e2e_tests/verify_*.sh; do \
			echo ''; echo '========== \$$(basename \$$f) =========='; \
			MODE=ssh REMOTE_HOST=remote bash \"\$$f\" lazyclaude || exit 1; \
		 done"
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml down

## VHS tape recording (e.g. make test-vhs TAPE=smoke)
test-vhs:
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml build
	TAPE=$(TAPE) docker compose -f vis_e2e_tests/docker-compose.ssh.yml run --rm vhs
	docker compose -f vis_e2e_tests/docker-compose.ssh.yml down

lint:
	golangci-lint run ./...

PREFIX ?= /usr/local

install: build
	install -d $(PREFIX)/bin
	install -m 755 scripts/lazyclaude-launch.sh $(PREFIX)/bin/$(BINARY)

clean:
	rm -rf bin/
