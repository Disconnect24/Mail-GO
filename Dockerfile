FROM golang:1.11.3-alpine3.8 as builder

# We assume only git is needed for all dependencies.
# openssl is already built-in.
RUN apk add -U --no-cache git
ENV GO111MODULE=on

WORKDIR /go/src/github.com/Disconnect24/Mail-GO
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy necessary parts of the Mail-GO source into builder's source
COPY *.go ./
COPY patch patch
COPY utilities utilities

# Build to name "app".
RUN CGO_ENABLED=0 go build -o app .

###########
# RUNTIME #
###########
FROM alpine:3.8

WORKDIR /go/src/github.com/Disconnect24/Mail-GO/
COPY --from=builder /go/src/github.com/Disconnect24/Mail-GO/ .

ENV DOCKERIZE_VERSION v0.6.1
RUN wget https://github.com/jwilder/dockerize/releases/download/$DOCKERIZE_VERSION/dockerize-alpine-linux-amd64-$DOCKERIZE_VERSION.tar.gz \
    && tar -C /usr/local/bin -xzvf dockerize-alpine-linux-amd64-$DOCKERIZE_VERSION.tar.gz \
    && rm dockerize-alpine-linux-amd64-$DOCKERIZE_VERSION.tar.gz && apk add -U --no-cache ca-certificates

# Wait until there's an actual MySQL connection we can use to start.
CMD ["dockerize", "-wait", "tcp://database:3306", "-timeout", "60s", "/go/src/github.com/Disconnect24/Mail-GO/app"]
