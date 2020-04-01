FROM golang:1.14-alpine as builder

# Setup
RUN mkdir -p /go/src/github.com/edenlabllc/traefik-tower
WORKDIR /go/src/github.com/edenlabllc/traefik-tower

# Copy & build
ADD . /go/src/github.com/edenlabllc/traefik-tower
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -installsuffix nocgo -o /traefik-tower .

# Copy into scratch container
FROM scratch
COPY --from=builder /traefik-tower ./
ENTRYPOINT ["./traefik-tower"]