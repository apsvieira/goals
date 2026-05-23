# Build frontend first
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Build backend (needs frontend dist)
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
# Copy frontend dist to where embed expects it
COPY --from=frontend /app/dist ./cmd/server/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -tags prod -ldflags "-X main.version=${VERSION}" -o server ./cmd/server

# Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /app/server ./server
EXPOSE 8080
CMD ["./server", "-db-type", "postgres"]
