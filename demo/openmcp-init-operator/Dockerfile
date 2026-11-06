# Build the manager binary
FROM golang:1.27-bookworm@sha256:ded31c68586d2e49e760acc2e65a884b23d032e9bbbed0ae0c55abd3fcaf4452 AS builder

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
#COPY pkg/ pkg/


# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-w -s' -o manager main.go

FROM gcr.io/distroless/static:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
USER 65532:65532

COPY --from=builder --chown=65532:65532  /workspace/manager /operator/manager

ENTRYPOINT ["/operator/manager"]
