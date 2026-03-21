##############################################
# Unified test Dockerfile for lazyclaude
#
# Targets:
#   test    — unit + integration tests (default)
#   remote  — sshd server for SSH E2E tests
#   local   — SSH test runner
#   vhs     — VHS tape recording
#
# Usage (from project root):
#   docker build -f tests/Dockerfile --target test -t lazyclaude-test .
#   docker build -f tests/Dockerfile --target vhs  -t lazyclaude-vhs .
#   docker compose -f tests/docker-compose.ssh.yml build
##############################################

# --- Stage: ephemeral SSH keypair ---
FROM alpine:latest AS keygen
RUN apk add --no-cache openssh-keygen
RUN ssh-keygen -t ed25519 -f /tmp/test_key -N "" -q

# --- Stage: shared base ---
FROM golang:latest AS base
RUN apt-get update && apt-get install -y --no-install-recommends \
    tmux openssh-server openssh-client curl nodejs expect git \
    && rm -rf /var/lib/apt/lists/*
ARG CLAUDE_VERSION=2.1.79
RUN ARCH=$(uname -m) && \
    case "$ARCH" in aarch64|arm64) PLATFORM="linux-arm64" ;; *) PLATFORM="linux-x64" ;; esac && \
    curl -fsSL "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases/${CLAUDE_VERSION}/${PLATFORM}/claude" \
    -o /usr/local/bin/claude && chmod +x /usr/local/bin/claude
ENV TERM=xterm-256color

# --- Stage: build all binaries ---
FROM base AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/lazyclaude ./cmd/lazyclaude && \
    go build -o bin/mock-claude-client ./cmd/mock-claude-client && \
    go build -o bin/gocui-demo ./vis_e2e_tests/minimal_gocui.go

# --- Target: test (unit + integration, replaces Dockerfile.test) ---
FROM base AS test
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=builder /app/bin/lazyclaude bin/lazyclaude
COPY --from=builder /app/bin/gocui-demo /usr/local/bin/gocui-demo
RUN ln -sf /app/scripts/lazyclaude-launch.sh /usr/local/bin/lazyclaude
COPY vis_e2e_tests/claude-settings.json /root/.claude/settings.json
CMD ["go", "test", "-cover", "./...", "-timeout", "120s"]

# --- Target: remote (sshd for SSH E2E) ---
FROM base AS remote
RUN ssh-keygen -A && mkdir -p /run/sshd /root/.ssh
COPY --from=keygen /tmp/test_key.pub /root/.ssh/authorized_keys
RUN chmod 700 /root/.ssh && chmod 600 /root/.ssh/authorized_keys
RUN sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && \
    echo 'GatewayPorts yes' >> /etc/ssh/sshd_config
COPY --from=builder /usr/local/bin/claude /usr/local/bin/claude
COPY --from=builder /app/bin/mock-claude-client /usr/local/bin/mock-claude-client
COPY vis_e2e_tests/claude-settings.json /root/.claude/settings.json
EXPOSE 22
CMD ["/usr/sbin/sshd", "-D"]

# --- Target: local (SSH test runner) ---
FROM base AS local
RUN mkdir -p /root/.ssh
COPY --from=keygen /tmp/test_key /root/.ssh/id_ed25519
RUN chmod 700 /root/.ssh && chmod 600 /root/.ssh/id_ed25519
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=builder /app/bin/lazyclaude bin/lazyclaude
RUN ln -sf /app/scripts/lazyclaude-launch.sh /usr/local/bin/lazyclaude
CMD ["go", "test", "-v", "-timeout", "120s", "./tests/", "-run", "TestE2E_SSH"]

# --- Target: vhs (VHS has its own base due to chromium/ttyd/ffmpeg deps) ---
FROM ghcr.io/charmbracelet/vhs:latest AS vhs
RUN apt-get update && apt-get install -y --no-install-recommends \
    tmux golang-go git nodejs curl openssh-client strace \
    && rm -rf /var/lib/apt/lists/*
ENV TERM=xterm-256color
RUN mkdir -p /root/.ssh
COPY --from=keygen /tmp/test_key /root/.ssh/id_ed25519
RUN chmod 700 /root/.ssh && chmod 600 /root/.ssh/id_ed25519
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/lazyclaude ./cmd/lazyclaude && \
    ln -sf /app/bin/lazyclaude /usr/local/bin/lazyclaude && \
    mkdir -p /app/outputs && \
    mv /bin/bash /bin/bash-real && \
    printf '#!/bin/bash-real\nif [ -n "$VHS_AUTO_TMUX" ] && [ -z "$VHS_IN_TMUX" ]; then\n  export VHS_IN_TMUX=1\n  exec tmux attach -t main\nfi\nexec /bin/bash-real "$@"\n' > /bin/bash && \
    chmod +x /bin/bash
ENTRYPOINT ["/app/vis_e2e_tests/entrypoint.sh"]
