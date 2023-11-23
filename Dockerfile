FROM golang:1.20-bullseye as build
WORKDIR /build-dir
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY internal internal
COPY cmd cmd
COPY pkg pkg

RUN go build -o /tmp/opentonapi github.com/tonkeeper/opentonapi/cmd/api


FROM ubuntu:20.04 as runner
RUN apt-get update && \
    apt-get install -y openssl ca-certificates libsecp256k1-dev && \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /go/pkg/mod/github.com/tonkeeper/tongo*/lib/linux /app/lib/
ENV LD_LIBRARY_PATH=/app/lib/
COPY --from=build /tmp/opentonapi /usr/bin/
CMD ["/usr/bin/opentonapi"]