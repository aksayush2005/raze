# Build stage
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY services/api/go.mod services/api/go.sum* ./
RUN go mod download
COPY services/api/ .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.20
RUN adduser -D -H raze
WORKDIR /app
COPY --from=build /out/api /out/migrate /usr/local/bin/
# Migrations are applied from the API image so the entrypoint can run them.
COPY database/migrations ./migrations
USER raze
EXPOSE 8080
CMD ["api"]
