FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tunnel-hub .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /tunnel-hub /usr/local/bin/tunnel-hub
EXPOSE 80 8080
ENTRYPOINT ["tunnel-hub"]
