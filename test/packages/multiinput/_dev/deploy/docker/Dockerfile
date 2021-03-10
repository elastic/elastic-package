FROM golang:1.15 as builder

ADD . /app

WORKDIR /app

RUN go mod download

ENV CGO_ENABLED=0
RUN go build

# ------------------------------------------------------------------------------
FROM scratch

COPY --chown=0:0 --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chown=0:0 --from=builder /app/stream /stream

CMD ["/stream"] 
