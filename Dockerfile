################################
## Build Stage ##

FROM golang:1.20 as builder
WORKDIR /jagger
COPY . /jagger

RUN CGO_ENABLED=0 go build -a -o jagger cmd/jaggerbot.go

################################
## Cert Stage ##

## Get CA certs 
FROM alpine:latest as certs
RUN apk --update add ca-certificates

################################
## Run Stage ##

FROM scratch 
## We need the certs because Scratch doesn't have them
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /jagger/jagger /
CMD ["/jagger"]