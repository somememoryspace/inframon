FROM harbor.dx.proxywerx.io/dockerhub/golang:alpine3.23@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166 AS builder

LABEL org.opencontainers.image.source=https://github.com/somememoryspace/inframon

WORKDIR /inframon

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inframon ./src/main.go

FROM docker.io/alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN apk add --no-cache ca-certificates openssl tzdata

WORKDIR /inframon

RUN mkdir -p /inframon/logs

COPY --from=builder /inframon/inframon .

ENV CONFIG_PATH=/config/config.yaml

RUN adduser -D -u 1000 linuxuser

RUN chown -R linuxuser:linuxuser /inframon && \
    chmod -R 755 /inframon

USER linuxuser

STOPSIGNAL SIGTERM

CMD ["/inframon/inframon", "--config=/config/config.yaml"]
