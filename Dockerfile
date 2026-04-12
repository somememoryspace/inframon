# Depends on Available px-containers base image: harbor.dx.proxywerx.io/library/go. Image is Pinned as of 04-2026. 
FROM harbor.dx.proxywerx.io/library/go@sha256:270ace163469d345591f57e122c04269dc65ae6d9c356fa721339e2f6fe9c8fb AS builder

LABEL org.opencontainers.image.source=https://github.com/somememoryspace/inframon

USER root

WORKDIR /inframon

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inframon ./src/main.go

# Depends on Available px-containers base image: harbor.dx.proxywerx.io/library/alpine-base. Image is Pinned as of 04-2026. 
FROM harbor.dx.proxywerx.io/library/alpine-base@sha256:0aa0489151ac24640444f88d21252c7cf309a8b41a22f949180c06b9747ea56c

RUN apk add --no-cache ca-certificates openssl tzdata

WORKDIR /inframon

RUN mkdir -p /inframon/logs

COPY --from=builder /inframon/inframon .

ENV CONFIG_PATH=/config/config.yaml

RUN chown -R linuxuser:linuxuser /inframon && \
    chmod -R 755 /inframon

USER linuxuser

STOPSIGNAL SIGTERM

CMD ["/apps/inframon", "--config=/config/config.yaml"]
