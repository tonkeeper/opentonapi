FROM docker.io/library/golang:1.22-bullseye AS gobuild
WORKDIR /build-dir
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY internal internal
COPY cmd cmd
COPY pkg pkg

RUN apt-get update && \
    apt-get install -y libsecp256k1-0 libsodium23
RUN go build -o /tmp/opentonapi github.com/tonkeeper/opentonapi/cmd/api

FROM ubuntu:20.04 as runner
RUN apt-get update && \
    apt-get install -y openssl ca-certificates libsecp256k1-0 libsodium23 wget && \
    rm -rf /var/lib/apt/lists/*
#COPY --from=build /go/pkg/mod/github.com/tonkeeper/tongo*/lib/linux /app/lib/
RUN mkdir -p /app/lib
RUN wget -O /app/lib/libemulator.so https://github.com/ton-blockchain/ton/releases/download/v2024.08/libemulator-linux-x86_64.so
ENV LD_LIBRARY_PATH=/app/lib/
COPY --from=gobuild /tmp/opentonapi /usr/bin/
CMD ["/usr/bin/opentonapi"]
