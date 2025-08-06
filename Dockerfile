FROM node:alpine3.22 AS frontend-builder
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

FROM alpine:3.22
WORKDIR /app
COPY --from=backend-builder /app/mediaz .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

ENTRYPOINT [ "./mediaz" ]
CMD ["serve"]
