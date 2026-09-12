FROM golang:1.26.5-bookworm AS builder

WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/capacity-manager ./cmd/capacity-manager
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/load-generator ./cmd/load-generator

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /out/capacity-manager /capacity-manager
COPY --from=builder /out/load-generator /load-generator

USER nonroot:nonroot
ENTRYPOINT ["/capacity-manager"]
