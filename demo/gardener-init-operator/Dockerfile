# Build the manager binary
FROM golang:1.26-bookworm AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY internal/ internal/
COPY cmd/ cmd/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-w -s' -o manager main.go

FROM gcr.io/distroless/static:nonroot
USER 65532:65532

COPY --from=builder --chown=65532:65532  /workspace/manager /operator/manager

ENTRYPOINT ["/operator/manager"]
