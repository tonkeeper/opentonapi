FROM golang:1.20 as build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY internal internal
COPY cmd cmd
COPY pkg pkg

RUN go build -o /tmp/opentonapi github.com/tonkeeper/opentonapi/cmd/api


FROM ubuntu as runner
COPY --from=gobuild /go/pkg/mod/github.com/tonkeeper/tongo*/lib/linux /app/lib/
ENV LD_LIBRARY_PATH=/app/lib/
COPY --from=build /tmp/opentonapi /usr/bin/
