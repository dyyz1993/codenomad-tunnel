FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /tunnel-hub .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates openssh-server

RUN mkdir -p /run/sshd && \
    ssh-keygen -A && \
    sed -i 's/#PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    echo "AllowUsers tunnel" >> /etc/ssh/sshd_config

RUN adduser -D -s /bin/ash tunnel && \
    mkdir -p /home/tunnel/.ssh && \
    chown tunnel:tunnel /home/tunnel/.ssh && \
    chmod 700 /home/tunnel/.ssh

COPY --from=builder /tunnel-hub /usr/local/bin/tunnel-hub
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 18080 18081 2222

VOLUME ["/data", "/home/tunnel/.ssh"]

ENTRYPOINT ["/entrypoint.sh"]
CMD ["--domain", "tunnel.example.com"]
