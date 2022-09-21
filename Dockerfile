FROM golang:1.19.1-alpine3.16
COPY . /go-work
WORKDIR /go-work/cmd/go-work
RUN cp /go-work/build/wait-for .
RUN go mod download
RUN go build
ENTRYPOINT ["./go-work"]