# syntax=docker/dockerfile:1.6
# Builder — pinned to an immutable digest (tag-poisoning resistant)
FROM golang:1.25-alpine@sha256:523c3effe300580ed375e43f43b1c9b091b68e935a7c3a92bfcc4e7ed55b18c2 AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
# Static binary: no libc, no dynamic loader — runs on distroless/static.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

# Runtime — distroless static: no shell, no package manager, no OS CVE surface,
# runs as the built-in nonroot user (uid 65532). Pinned by digest.
# checkov:skip=CKV_DOCKER_2: health is owned by Kubernetes probes; distroless has no shell for a HEALTHCHECK anyway
FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639

COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
