FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o mediaz main.go

FROM alpine:latest
WORKDIR /app
COPY --from=backend-builder /app/mediaz .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
CMD ["./mediaz", "serve"]
