FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/cloud-sd ./cmd/cloud-sd

FROM gcr.io/distroless/static-debian12:nonroot

ENV GIN_MODE=release

WORKDIR /
COPY --from=builder /out/cloud-sd /cloud-sd
COPY examples/config.yaml /examples/config.yaml

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/cloud-sd"]
CMD ["-config", "/examples/config.yaml"]
