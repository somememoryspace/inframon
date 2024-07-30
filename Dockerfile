FROM golang:1.20 AS builder

WORKDIR /inframon

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o inframon .

FROM debian:bullseye-slim

WORKDIR /inframon

COPY --from=builder /inframon/inframon .

CMD ["./inframon"]