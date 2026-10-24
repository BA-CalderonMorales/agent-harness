# syntax=docker/dockerfile:1

# Multi-stage build for the agent-harness image.
#
# The image is also the runtime OpenShell expects: an unprivileged
# "sandbox" user, a writable /workspace, and the tools the agent's shell
# needs (bash, git, CA certificates). Everything the harness itself does
# is compiled in; there is no Node or Python toolchain by design.

FROM golang:1.26.9-bookworm AS build

WORKDIR /src

# Module download first so source edits reuse the layer cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.GitSHA=${GIT_SHA} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/agent-harness ./cmd/agent-harness

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/sandbox \
    XDG_CONFIG_HOME=/home/sandbox/.config \
    XDG_DATA_HOME=/home/sandbox/.local/share

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
        git \
        openssh-client \
    && rm -rf /var/lib/apt/lists/*

# OpenShell's policy defaults expect an unprivileged "sandbox" user.
RUN useradd --create-home --uid 1000 --shell /bin/bash sandbox \
    && mkdir -p /workspace \
    && chown sandbox:sandbox /workspace

COPY --from=build /out/agent-harness /usr/local/bin/agent-harness

USER sandbox
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/agent-harness"]
