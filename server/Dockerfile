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

# Aliases so `docker exec jotnar updateinference` works (without repeating "jotnar").
RUN printf '#!/bin/sh\nexec jotnar updateinference "$@"\n' > /usr/local/bin/updateinference && chmod +x /usr/local/bin/updateinference
RUN printf '#!/bin/sh\nexec jotnar pairingcode "$@"\n' > /usr/local/bin/pairingcode && chmod +x /usr/local/bin/pairingcode
RUN printf '#!/bin/sh\nexec jotnar debuglog "$@"\n' > /usr/local/bin/debuglog && chmod +x /usr/local/bin/debuglog

EXPOSE 8910
VOLUME /data
ENTRYPOINT ["/entrypoint.sh"]
