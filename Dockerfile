# ============= Donerka House Build =============
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o donerka-server .

# ============= Final minimal image =============
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary and assets
COPY --from=builder /app/donerka-server .
COPY --from=builder /app/data/ ./data/
COPY --from=builder /app/static/ ./static/

EXPOSE 8080

ENV PORT=8080

CMD ["./donerka-server"]
