# ETAP 1: Builder
# Używamy zmiennej BUILDPLATFORM, aby pobrać obraz zgodny z maszyną budującą
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

# Dodanie zmiennej pozwalającej na cross-compilation (wstrzykiwana przez buildx)
ARG TARGETARCH

# Instalacja certyfikatów SSL oraz stworzenie użytkownika nieuprzywilejowanego
RUN apk --no-cache add ca-certificates && adduser -D -g '' -H -s /sbin/nologin pawchouser

WORKDIR /app
COPY main.go .

# Inicjalizacja modułu i kompilacja
# Optymalizacja wagowa: -s (strip symbol table) -w (strip DWARF)
# GOARCH=$TARGETARCH pozwala na poprawną budowę na podaną platformę np. linux/arm64
RUN go mod init weatherapp && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /app/server main.go

# ETAP 2: Docelowy obraz produkcyjny
FROM scratch

# Standaryzowane etykiety OCI
LABEL org.opencontainers.image.authors="Jakub Fus"
LABEL org.opencontainers.image.title="Aplikacja Pogodowa PAwChO"

# 1 warstwa docelowa: Certyfikaty CA z buildera (wymagane dla zapytań HTTPS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 2 warstwa docelowa: Plik /etc/passwd zawierający naszego użytkownika
COPY --from=builder /etc/passwd /etc/passwd

# 3 warstwa docelowa: Skompilowana aplikacja binarna
COPY --from=builder /app/server /server

# Przełączenie na użytkownika non-root ze względów bezpieczeństwa
USER pawchouser

# Aplikacja działa na porcie 8080
EXPOSE 8080

# Healthcheck wywołujący aplikację z parametrem "check"
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/server", "check"]

# Punkt wejścia
ENTRYPOINT ["/server"]