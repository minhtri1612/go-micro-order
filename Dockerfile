# Builder = Jenkins host (t4g arm64). QEMU+Go SIGSEGVs on go mod download.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /service .

# Final image follows docker build --platform (linux/amd64 for Kind).
FROM alpine:latest
WORKDIR /app
COPY --from=builder /service .
EXPOSE 8081
CMD ["./service"]
