FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o jotnar ./cmd/jotnar

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl sqlite
COPY --from=builder /build/jotnar /usr/local/bin/jotnar
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8910
VOLUME /data
ENTRYPOINT ["/entrypoint.sh"]
