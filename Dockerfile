FROM golang:1.22-alpine AS builder

WORKDIR /app
RUN go env -w GOPROXY=https://goproxy.cn,direct
COPY main.go .
COPY public/ ./public/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o proxy main.go

FROM alpine:latest
WORKDIR /app


COPY --from=builder /app/proxy .

EXPOSE 8080
ENTRYPOINT ["/app/proxy"]
