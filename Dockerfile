FROM golang:1.22.5-alpine AS builder

WORKDIR /inframon

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inframon ./src/main.go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates openssl

WORKDIR /inframon

RUN mkdir -p /inframon/logs

COPY --from=builder /inframon/inframon .

ENV CONFIG_PATH=/config/config.yaml

RUN adduser -D -u 30000 linuxuser

RUN chown -R linuxuser:linuxuser /inframon && \
    chmod -R 755 /inframon

USER linuxuser

STOPSIGNAL SIGTERM

CMD ["/inframon/inframon", "--config=/config/config.yaml"]