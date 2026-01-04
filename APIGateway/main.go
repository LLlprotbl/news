package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type Config struct {
	NewsService     string `json:"news_service"`
	CommentsService string `json:"comments_service"`
	Port            string `json:"port"`
}

var cfg Config

func init() {
	cfg = Config{
		NewsService:     "http://localhost:80",
		CommentsService: "http://localhost:8081",
		Port:            ":8080",
	}
}

// модели
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

// структура для канала
type serviceResult struct {
	data interface{}
	err  error
}

// ------------------------------------------------------------------
// Метод вывода списка новостей
// GET /news
func getNewsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Запрашиваем 10 последних новостей у сервиса новостей
	resp, err := http.Get(cfg.NewsService + "/news/10")
	if err != nil {
		http.Error(w, "news service error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var news []NewsShortDetailed
	if err := json.NewDecoder(resp.Body).Decode(&news); err != nil {
		http.Error(w, "error decoding news", http.StatusInternalServerError)
		return
	}
	writeJSON(w, news)
}

// Метод фильтра новостей
// GET /news/filter?date=2025-12-18
// пример http://localhost:8080/news/filter?s=редакции
func filterNews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем поисковое слово из запроса пользователя
	searchQuery := r.URL.Query().Get("s")
	if searchQuery == "" {
		http.Error(w, "s query parameter is required", http.StatusBadRequest)
		return
	}

	// Пробрасываем запрос в News Service
	safeQuery := url.QueryEscape(searchQuery)
	targetURL := fmt.Sprintf("%s/news/search?s=%s", cfg.NewsService, safeQuery)
	resp, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "news service error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var news []NewsShortDetailed
	if err := json.NewDecoder(resp.Body).Decode(&news); err != nil {
		http.Error(w, "error decoding news", http.StatusInternalServerError)
		return
	}

	writeJSON(w, news)
}

// Метод получения детальной новости
// GET /news/{id}
func getNewsDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id") // Получаем ID новости из запроса
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Канал с буфером = количеству запросов
	resChan := make(chan serviceResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// Запрос к сервису новостей
	go func() {
		defer wg.Done()

		// Формируем URL сервис новостей на :80 имеет эндпоинт /news/detail
		url := fmt.Sprintf("http://localhost:80/news/detail/%s", id)
		// Выполняем запрос
		resp, err := http.Get(url)
		if err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		// Закрываем поток после завершения функции
		defer resp.Body.Close()
		// JSON из resp.Body в слайс []Comment
		var news NewsFullDetailed
		// Создаем декодер из тела ответа и сразу вызываем расшифровку
		err = json.NewDecoder(resp.Body).Decode(&news)
		if err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		// Мы отправляем в канал данные с типом NewsFullDetailed
		resChan <- serviceResult{data: news}
	}()

	// Горутина для сервиса комментариев (SQLite)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("http://localhost:8081/comments?news_id=%s", id)
		resp, err := http.Get(url)
		if err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		defer resp.Body.Close()
		var comments []Comment
		if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
			resChan <- serviceResult{err: err}
			return
		}
		resChan <- serviceResult{data: comments}
	}()

	// Ждём все горутины ===
	wg.Wait()
	close(resChan)

	// Собираем итоговый результат ===
	var result NewsFullDetailed

	for res := range resChan {
		if res.err != nil {
			// Если хотя бы один запрос упал — возвращаем ошибку
			http.Error(w, "service error", http.StatusInternalServerError)
			return
		}

		switch v := res.data.(type) {
		case NewsFullDetailed:
			// Копируем данные новости (Title, Content, CreatedAt и т.д.)
			result.ID = v.ID
			result.Title = v.Title
			result.Content = v.Content
			result.PubTime = v.PubTime
		case []Comment:
			// Добавляем массив комментариев
			result.Comments = v
		}
	}

	writeJSON(w, result)
}

// Метод добавления комментария
// POST /comments
func addComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Отправляем POST запрос в CommentsService
	// Мы просто передаем r.Body дальше
	resp, err := http.Post("http://localhost:8081/comments", "application/json", r.Body)
	if err != nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	// Теперь нужно прочитать ответ от сервиса и отдать его пользователю
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	writeJSON(w, result)
}

// функция ответа
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(data)
}

//  http сервер

func main() {
	http.HandleFunc("/news", getNewsList)
	http.HandleFunc("/news/filter", filterNews)
	http.HandleFunc("/news/detail", getNewsDetail)
	http.HandleFunc("/comment/add", addComment)

	fmt.Println("API Gateway запущен на http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
