# Build the node-harvester controller
FROM golang:alpine as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY pkg/controller/ pkg/controller/
COPY pkg/common/ pkg/common/
COPY pkg/drainer/ pkg/drainer/
COPY pkg/types/ pkg/types/
COPY pkg/supervisor/ pkg/supervisor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o node-harvester main.go

# Use distroless as minimal base image to package the node-harvester binary
FROM scratch
WORKDIR /
COPY --from=builder /workspace/node-harvester .

ENTRYPOINT ["/node-harvester"]
