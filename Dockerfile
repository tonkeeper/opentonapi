FROM golang:1.20 as build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY internal internal
COPY cmd cmd
COPY pkg pkg
RUN go build -o /tmp/opentonapi github.com/tonkeeper/opentonapi/cmd/api


FROM ubuntu as runner
COPY --from=build /tmp/opentonapi /usr/bin/
