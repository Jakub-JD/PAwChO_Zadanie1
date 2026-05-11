# PAwChO_Zadanie1_Dodatwkowe (Poziom 3)
**Autor:** Jakub Fus

**GitHub:** https://github.com/Jakub-JD/PAwChO_Zadanie1

**DockerHub:** https://hub.docker.com/repository/docker/jakubjd/weatherapp/general

## 1. Zmieniony plik Dockerfile_dod
```
# syntax=docker/dockerfile:1.4
# Powyższa linia aktywuje rozszerzony frontend BuildKit 

# ETAP 1: Builder
# Używamy zmiennej BUILDPLATFORM, aby pobrać obraz zgodny z maszyną budującą
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Dodanie zmiennej pozwalającej na cross-compilation (wstrzykiwana przez buildx)
ARG TARGETARCH

# Instalacja certyfikatów SSL oraz stworzenie użytkownika nieuprzywilejowanego
RUN apk --no-cache add ca-certificates && adduser -D -g '' -H -s /sbin/nologin pawchouser

WORKDIR /app

# Funkcjonalność mount secret 
# Demonstrujemy bezpieczne użycie sekretu (np. klucza API lub tokena) podczas budowy
RUN --mount=type=secret,id=my_token,required=false \
    if [ -f /run/secrets/my_token ]; then \
        echo "Budowanie z użyciem autoryzacji..."; \
    else \
        echo "Budowanie publiczne..."; \
    fi

# Kopiowanie kodu (w komendzie build wskażemy GitHub jako kontekst)
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
LABEL org.opencontainers.image.title="Aplikacja Pogodowa PAwChO - Wersja Rozszerzona"

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
```
## 2. Utworzenie nowego buildera ze sterownikiem docker-container

<img width="1064" height="1005" alt="Zrzut ekranu 2026-05-11 222338" src="https://github.com/user-attachments/assets/ed1a4239-df66-42f8-a86e-475186bd806c" />

<br><br>

## 3. Zbudowanie obrazu prosto z repozytorium github'a z użyciem cache registry

- Pierwsze budowanie
<img width="1247" height="1263" alt="Zrzut ekranu 2026-05-11 224003" src="https://github.com/user-attachments/assets/47ade505-767b-4867-a9bc-fab149bfeee8" />

<br><br>
- Kolejne budowanie
<img width="1311" height="1215" alt="image" src="https://github.com/user-attachments/assets/10afd4a1-7ffe-4d65-92bd-0baa693525e5" />

<br><br>
Użyte parametry w komendzie:
* `--cache-to type=registry,ref=jakubjd/weatherapp:cache,mode=max`
* `--cache-from type=registry,ref=jakubjd/weatherapp:cache`

**Wyniki optymalizacji (Analiza logów):**

1.  **Pierwsze budowanie (Zimny start):**
    Proces zajął **~250 sekund**. Silnik BuildKit pobrał wszystkie obrazy bazowe, wykonał pełną kompilację kodu (`go build`) dla dwóch architektur (amd64 i arm64), a następnie wyeksportował wynik oraz warstwy pośrednie do repozytorium `weatherapp:cache` (tryb `max`).

2.  **Kolejne budowanie (Z wykorzystaniem Cache):**
    Jak widać na załączonym zrzucie ekranu logów z drugiego uruchomienia, czas budowania spadł do zaledwie **9.4 sekundy**.
    
    * **Dowód działania:** W logach widoczne są adnotacje `CACHED` przy najbardziej czasochłonnych krokach, takich jak pobieranie zależności (`go mod init`) oraz sama kompilacja skrośna (`go build`).
    * **Mechanizm:** Zamiast wykonywać instrukcje lokalnie, silnik `docker-container` pobrał (zaimportował) gotowe manifesty z repozytorium `:cache` (`=> importing cache manifest from jakubjd/weatherapp:cache`).

**Wniosek:** Zastosowanie zewnętrznego mechanizmu `registry cache` diametralnie skraca czas budowania aplikacji w wieloetapowych środowiskach CI/CD.


## 4. Sprawdzenie manifestu wieloplatformowego (amd64 i arm64)

<img width="1540" height="672" alt="Zrzut ekranu 2026-05-11 224221" src="https://github.com/user-attachments/assets/dc5ee347-13fd-42ac-a559-acf20d217b20" />


<br><br>


## 5. Analiza podatności CVE
```
jakub@LaptopJF:~/Zad_1$ docker run --rm aquasec/trivy image jakubjd/weatherapp:dodatkowe
2026-05-11T20:56:56Z    INFO    [vulndb] Need to update DB
2026-05-11T20:56:56Z    INFO    [vulndb] Downloading vulnerability DB...
2026-05-11T20:56:56Z    INFO    [vulndb] Downloading artifact...        repo="mirror.gcr.io/aquasec/trivy-db:2"
95.25 KiB / 92.60 MiB [>_____________________________________________________________] 0.10% ? p/s ?
...
100.00% 899.17 KiB p/s ETA 0s92.60 MiB / 92.60 MiB [-------------------------------------------------] 100.00% 1.22 MiB p/s 1m16s2026-05-11T20:58:13Z      INFO    [vulndb] Artifact successfully downloaded      repo="mirror.gcr.io/aquasec/trivy-db:2"
2026-05-11T20:58:13Z    INFO    [vuln] Vulnerability scanning is enabled
2026-05-11T20:58:13Z    INFO    [secret] Secret scanning is enabled
2026-05-11T20:58:13Z    INFO    [secret] If your scanning is slow, please try '--scanners vuln' to disable secret scanning
2026-05-11T20:58:13Z    INFO    [secret] Please see https://trivy.dev/docs/v0.70/guide/scanner/secret#recommendation for faster secret detection
2026-05-11T20:58:18Z    INFO    Number of language-specific files       num=1
2026-05-11T20:58:18Z    INFO    [gobinary] Detecting vulnerabilities...

Report Summary

┌────────┬──────────┬─────────────────┬─────────┐
│ Target │   Type   │ Vulnerabilities │ Secrets │
├────────┼──────────┼─────────────────┼─────────┤
│ server │ gobinary │        0        │    -    │
└────────┴──────────┴─────────────────┴─────────┘
Legend:
- '-': Not scanned
- '0': Clean (no security findings detected)
```
| Skopiowałem terminal kasując raportowanie postępu testu zamiast robić zrzut ekranu ponieważ dużą część ekranu zajmował postęp wykonywania testu przez co załapanie komendy oraz wyniku końcowego testu na jednym zrzucie było nie możliwe

Widzimy 0 zagrożeń co osiągnąłem dopiero po zmianie głównego obrazu golang z wersji 1.22 na 1.25. Przed zmianą otrzymałem dużą ilość błędów, w tym 1 Critical i aż 14 High, natomiast wszystkie błędy dotyczyły biblioteki stdlib (standarowej języka Go), wersja 1.22 okazała się przestarzałą lecz zmiana na 1.25 naprawiła wszystko.



