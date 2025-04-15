# Stage 1: Build
FROM golang:1.24.2 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy
COPY . .
# Build sa optimizacijama: bez CGO, ukloni debug informacije
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# Stage 2: Run
FROM alpine:3.20 AS final
# Dodaj ca-certificates ako servis koristi HTTPS
RUN apk add --no-cache ca-certificates
# Kopiraj binarni fajl
COPY --from=build /app/server /server
# Postavi non-root korisnika radi sigurnosti
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
# Eksponiraj port (poklapa se sa docker-compose.yml)
EXPOSE 80
ENTRYPOINT ["/server"]