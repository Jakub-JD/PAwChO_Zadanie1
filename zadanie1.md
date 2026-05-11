# PAwChO_Zadanie1
**Autor:** Jakub Fus

**GitHub:** https://github.com/Jakub-JD/PAwChO_Zadanie1

**DockerHub:** https://hub.docker.com/repository/docker/jakubjd/weatherapp/general

## 1. Kod oprogramowania
Aplikacja została napisana w języku Go. Realizuje ona serwer HTTP, który przy starcie loguje dane autora oraz nasłuchuje na porcie 8080. Aplikacja pobiera dane pogodowe z zewnętrznego API (Open-Meteo) dla dwóch wybranych lokalizacji: Lublin i Kraków.

```
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	PORT   = "8080"
	AUTHOR = "Jakub Fus"
)

// Struktura na dane z darmowego API open-meteo.com
type OpenMeteoResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
	} `json:"current_weather"`
}

func main() {
	// Wbudowany mechanizm na potrzeby Docker HEALTHCHECK
	if len(os.Args) > 1 && os.Args[1] == "check" {
		res, err := http.Get("http://127.0.0.1:" + PORT + "/health")
		if err != nil || res.StatusCode != http.StatusOK {
			os.Exit(1) // Healthcheck fail
		}
		os.Exit(0) // Healthcheck ok
	}

	// 1a. Wymagane logi po starcie serwera
	fmt.Println("========================================")
	fmt.Printf(" [LOG] Data uruchomienia: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf(" [LOG] Autor oprogramowania: %s\n", AUTHOR)
	fmt.Printf(" [LOG] Aplikacja nasłuchuje na porcie: %s\n", PORT)
	fmt.Println("========================================")

	// Routing
	http.HandleFunc("/", renderUI)
	http.HandleFunc("/api/weather", getWeather)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // endpoint dla healthchecka
	})

	err := http.ListenAndServe(":"+PORT, nil)
	if err != nil {
		fmt.Printf("Błąd serwera: %v\n", err)
	}
}

// Generowanie prostego interfejsu 
func renderUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="pl">
<head>
    <meta charset="UTF-8">
    <title>Stacja Pogodowa PAwChO</title>
</head>
<body>
    <h2>Sprawdź aktualną pogodę</h2>
    <select id="citySelect">
        <option value="Lublin">Lublin</option>
        <option value="Krakow">Kraków</option>
    </select>
    <button onclick="fetchW()">Pokaż</button>
    <div id="weatherResult"></div>

    <script>
        async function fetchW() {
            const city = document.getElementById('citySelect').value;
            const resDiv = document.getElementById('weatherResult');
            resDiv.innerText = "Pobieranie...";
            try {
                const req = await fetch('/api/weather?city=' + city);
                const data = await req.json();
                const cityName = city === 'Krakow' ? 'Kraków' : 'Lublin';
                resDiv.innerText = cityName + " | Temp: " + data.temp + "°C | Wiatr: " + data.wind + " km/h";
            } catch(e) {
                resDiv.innerText = "Nie udało się pobrać danych.";
            }
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
// Logika pobierania pogody
func getWeather(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	
	// Koordynaty dla miast
	lat, lon := "51.25", "22.56" // Domyślnie Lublin
	if city == "Krakow" {
		lat, lon = "50.06", "19.93"
	}

	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&current_weather=true", lat, lon)
	
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Błąd zewn. API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var apiRes OpenMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		http.Error(w, "Błąd parsowania JSON", http.StatusInternalServerError)
		return
	}

	// Odpowiedź JSON dla naszego front-endu
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"temp":"%.1f", "wind":"%.1f"}`, 
		apiRes.CurrentWeather.Temperature, 
		apiRes.CurrentWeather.Windspeed)
}
```

## 2. Zawartość pliku Dockerfile
```
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
```
## 3. Polecenia do pracy z Dockerfile i obrazem :

a. zbudowania opracowanego obrazu kontenera, 
- `docker buildx build --platform linux/amd64,linux/arm64 -t jakubjd/weatherapp:latest --no-cache --push` .

<img width="1238" height="1125" alt="image" src="https://github.com/user-attachments/assets/a457804d-b838-407c-acf4-d4a5644c9fcc" />
<br> 
<br>


b. uruchomienia kontenera na podstawie zbudowanego obrazu, 
- `docker run -d --name weather_container -p 8080:8080 jakubjd/weatherapp:latest`

<img width="1260" height="123" alt="image" src="https://github.com/user-attachments/assets/0c53947d-6cf3-42ad-9423-7087a320e76a" />
<br> 
<br>


c. sposobu uzyskania informacji z logów, które wygenerowałą opracowana aplikacja podczas 
uruchamiana kontenera (patrz: punkt 1a), 
- `docker logs weather_container`

<img width="781" height="213" alt="image" src="https://github.com/user-attachments/assets/85106962-0d55-4db2-b225-42796a013b77" />
<br> 
<br>


d. sprawdzenia, ile warstw posiada zbudowany obraz oraz jaki jest rozmiar obrazu. 
- `docker images jakubjd/weatherapp:latest` (rozmiar)
- `docker history jakubjd/weatherapp:latest` (liczba warstw)

lub jeśli chcemy zwrócenia po prostu dokładnej liczby warstw
- `docker image inspect jakubjd/weatherapp:latest | jq '.[0].RootFS.Layers | length'`

<img width="1269" height="891" alt="image" src="https://github.com/user-attachments/assets/c48a5d93-8a5f-4964-8aaf-cd8f67394567" />
<br> 
<br>



## Działanie aplikacji

<img width="2557" height="1439" alt="image" src="https://github.com/user-attachments/assets/d9beecc1-4824-4208-8571-0637e7ee62ed" />

<img width="2559" height="1439" alt="image" src="https://github.com/user-attachments/assets/1af68111-4357-4502-a619-8ba42cafebdb" />



