FROM golang:1.10 as builder

COPY . /go/src/github.com/JulienBalestra/kube-lock

RUN make -C /go/src/github.com/JulienBalestra/kube-lock re

FROM busybox:latest

COPY --from=builder /go/src/github.com/JulienBalestra/kube-lock/kube-lock /usr/local/bin/kube-lock
