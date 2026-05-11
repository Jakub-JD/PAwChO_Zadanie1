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

// Generowanie prostego interfejsu (tylko 2 miasta)
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