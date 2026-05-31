# Depends on Available px-containers base image: harbor.dx.proxywerx.io/library/go. Image is Pinned as of 04-2026. 
FROM harbor.dx.proxywerx.io/library/go@sha256:6fa42a880bba82b88fafdd7e9d7b42073f668f28d1773051982db188f1c3e2be AS builder

LABEL org.opencontainers.image.source=https://github.com/somememoryspace/inframon

USER root

WORKDIR /inframon

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inframon ./src/main.go

# Depends on Available px-containers base image: harbor.dx.proxywerx.io/library/alpine-base. Image is Pinned as of 04-2026. 
FROM harbor.dx.proxywerx.io/library/alpine-base@sha256:dff90e4bce42c0399af3700e741e4ea1e3dcca2f0198de892b3b0e9b8694840c

RUN apk add --no-cache ca-certificates openssl tzdata

WORKDIR /inframon

RUN mkdir -p /app/inframon/logs

COPY --from=builder /inframon/inframon /app/inframon/inframon

ENV CONFIG_PATH=/config/config.yaml

RUN chown -R linuxuser:linuxuser /app/inframon && \
    chmod -R 755 /app/inframon

USER linuxuser

STOPSIGNAL SIGTERM

CMD ["/app/inframon/inframon", "--config=/config/config.yaml"]
