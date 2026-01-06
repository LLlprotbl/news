package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Middleware для логов и ID
func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.URL.Query().Get("request_id")
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] CENSOR | %s %s | ID: %s | DUR: %v",
			time.Now().Format("15:04:05"), r.Method, r.URL.Path, reqID, time.Since(start))
	})
}

func handleCensor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Наша логика цензуры
	badWords := []string{"qwerty", "йцукен", "zxvbnm"}
	for _, word := range badWords {
		if strings.Contains(strings.ToLower(body.Text), word) {
			http.Error(w, "Inappropriate content", http.StatusBadRequest) // 400 ошибка
			return
		}
	}

	w.WriteHeader(http.StatusOK) // 200 OK
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/censor", handleCensor)

	fmt.Println("Censor Service запущен на :8082")
	log.Fatal(http.ListenAndServe(":8082", loggerMiddleware(mux)))
}
