FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/server

# nonroot: runs as UID 65532 with no shell.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
