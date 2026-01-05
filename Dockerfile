# Build backend
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -tags prod -o server ./cmd/server

# Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /app/server ./server
COPY --from=frontend /app/dist ./dist
EXPOSE 8080
CMD ["./server", "-db-type", "postgres"]
