FROM golang:1.22.5 AS builder

WORKDIR /inframon

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inframon ./src/main.go

FROM debian:bullseye-slim

RUN apt-get update -y && \
    apt-get install -y ca-certificates openssl libcap2-bin && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m inframon

WORKDIR /inframon

RUN mkdir -p /inframon/logs && chown inframon:inframon /inframon/logs

COPY --from=builder /inframon/inframon .

RUN setcap cap_net_raw,cap_net_admin+eip ./inframon

USER inframon

ENV CONFIG_PATH=/config/config.yaml

CMD ["sh", "-c", "./inframon --config=$CONFIG_PATH"]