package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	NewsService     string `json:"news_service"`
	CommentsService string `json:"comments_service"`
	Port            string `json:"port"`
}

var cfg Config

func init() {
	cfg = Config{
		NewsService:     "http://localhost:80",   // Сервис новостей
		CommentsService: "http://localhost:8081", // Сервис комментариев
		Port:            ":8080",
	}
}

// Структуры ответа

type NewsResponse struct {
	News       []NewsShortDetailed `json:"news"`
	Pagination Pagination          `json:"pagination"`
}

type Pagination struct {
	TotalPages   int `json:"total_pages"`
	CurrentPage  int `json:"current_page"`
	ItemsPerPage int `json:"items_per_page"`
}

type NewsShortDetailed struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	PubTime int64  `json:"pub_time"`
	Link    string `json:"link"`
}

type NewsFullDetailed struct {
	ID       int       `json:"id"`
	Title    string    `json:"title"`
	Content  string    `json:"content"`
	PubTime  int64     `json:"pub_time"`
	Comments []Comment `json:"comments"`
}

type Comment struct {
	ID        int    `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type serviceResult struct {
	data interface{}
	err  error
}

// Middleware
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.URL.Query().Get("request_id")
		if len(reqID) < 6 {
			reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(r.Context(), "request_id", reqID)
		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		log.Printf("[%s] GATEWAY | %s %s | STATUS: %d | ID: %s | DUR: %v",
			time.Now().Format("15:04:05"), r.Method, r.URL.Path, reqID, rw.statusCode, time.Since(start))
	})
}

// Обработчики

// GET /news
func handleNews(w http.ResponseWriter, r *http.Request) {
	reqID, _ := r.Context().Value("request_id").(string)
	s := r.URL.Query().Get("s")
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	// Формируем URL к микросервису новостей
	targetURL := fmt.Sprintf("%s/news?s=%s&page=%s&request_id=%s",
		cfg.NewsService, url.QueryEscape(s), page, reqID)

	resp, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "news service unreachable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var data NewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		http.Error(w, "error decoding news", http.StatusInternalServerError)
		return
	}
	writeJSON(w, data)
}

func getNewsDetail(w http.ResponseWriter, r *http.Request) {
	reqID, _ := r.Context().Value("request_id").(string)
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	resChan := make(chan serviceResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// Запрос к новостям
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("%s/news/%s?request_id=%s", cfg.NewsService, id, reqID)
		resp, err := http.Get(url)
		if err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		defer resp.Body.Close()
		var n NewsFullDetailed
		json.NewDecoder(resp.Body).Decode(&n)
		resChan <- serviceResult{data: n}
	}()

	// Запрос к комментариям
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("%s/comments?news_id=%s&request_id=%s", cfg.CommentsService, id, reqID)
		resp, err := http.Get(url)
		if err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		defer resp.Body.Close()
		var c []Comment
		json.NewDecoder(resp.Body).Decode(&c)
		resChan <- serviceResult{data: c}
	}()

	wg.Wait()
	close(resChan)

	var result NewsFullDetailed
	for res := range resChan {
		if res.err != nil {
			http.Error(w, "internal service error", http.StatusInternalServerError)
			return
		}
		switch v := res.data.(type) {
		case NewsFullDetailed:
			result.ID, result.Title, result.Content, result.PubTime = v.ID, v.Title, v.Content, v.PubTime
		case []Comment:
			result.Comments = v
		}
	}
	writeJSON(w, result)
}

func handleAddComment(w http.ResponseWriter, r *http.Request) {
	reqID, _ := r.Context().Value("request_id").(string)
	// Читаем тело комментария
	var commentData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&commentData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Превращаем обратно в байты для отправки в сервисы
	bodyBytes, _ := json.Marshal(commentData)

	// СИНХРОННЫЙ ЗАПРОС К ЦЕНЗОРУ
	censorURL := fmt.Sprintf("http://localhost:8082/censor?request_id=%s", reqID)
	// Создаем новый запрос, так как r.Body уже прочитан
	censorResp, err := http.Post(censorURL, "application/json", strings.NewReader(string(bodyBytes)))

	if err != nil || censorResp.StatusCode != http.StatusOK {
		http.Error(w, "Comment failed censorship", http.StatusBadRequest)
		return
	}

	// ЕСЛИ ЦЕНЗОР ОДОБРИЛ (200 OK) — ОТПРАВЛЯЕМ В СЕРВИС КОММЕНТАРИЕВ
	commentsURL := fmt.Sprintf("%s/comments?request_id=%s", cfg.CommentsService, reqID)
	resp, err := http.Post(commentsURL, "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		http.Error(w, "Comments service unreachable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	writeJSON(w, res)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/news", handleNews)
	mux.HandleFunc("/news/detail", getNewsDetail)
	mux.HandleFunc("/comment/add", handleAddComment)

	wrappedMux := loggerMiddleware(mux)

	fmt.Println("API Gateway запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", wrappedMux))
}
