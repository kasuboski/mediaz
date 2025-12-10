FROM --platform=$BUILDPLATFORM node:alpine3.22 AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS go-cross-compiler

ARG TARGETARCH
ENV GOOS=linux
ENV GOARCH=${TARGETARCH}
ENV CGO_ENABLED=1

# Install Zig for CGO cross-compilation
RUN apk add --no-cache wget tar xz && \
    wget -q https://ziglang.org/download/0.13.0/zig-linux-$(uname -m)-0.13.0.tar.xz && \
    tar -xf zig-linux-$(uname -m)-0.13.0.tar.xz && \
    mv zig-linux-$(uname -m)-0.13.0 /usr/local/zig && \
    rm zig-linux-$(uname -m)-0.13.0.tar.xz

# Set Zig as the C/C++ compiler for the target architecture
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        echo '#!/bin/sh' > /usr/local/bin/gcc && \
        echo 'exec /usr/local/zig/zig cc -target x86_64-linux-musl "$@"' >> /usr/local/bin/gcc && \
        echo '#!/bin/sh' > /usr/local/bin/g++ && \
        echo 'exec /usr/local/zig/zig c++ -target x86_64-linux-musl "$@"' >> /usr/local/bin/g++ && \
        chmod +x /usr/local/bin/gcc /usr/local/bin/g++; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        echo '#!/bin/sh' > /usr/local/bin/gcc && \
        echo 'exec /usr/local/zig/zig cc -target aarch64-linux-musl "$@"' >> /usr/local/bin/gcc && \
        echo '#!/bin/sh' > /usr/local/bin/g++ && \
        echo 'exec /usr/local/zig/zig c++ -target aarch64-linux-musl "$@"' >> /usr/local/bin/g++ && \
        chmod +x /usr/local/bin/gcc /usr/local/bin/g++; \
    fi

FROM --platform=$BUILDPLATFORM go-cross-compiler AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o mediaz main.go

FROM alpine:3.22
WORKDIR /app
COPY --from=backend-builder /app/mediaz .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

ENTRYPOINT [ "./mediaz" ]
CMD ["serve"]
