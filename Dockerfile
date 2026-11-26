
ARG BASE_IMAGE=golang:1.26
FROM ${BASE_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x


COPY . .

# Limit concurrent package builds to reduce peak memory on small builders.
ARG GO_BUILD_PARALLELISM=1

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -p=${GO_BUILD_PARALLELISM} -v -o manager cmd/main.go


FROM registry.cn-hangzhou.aliyuncs.com/mingyuan_cloud_native/distroless-static/nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
