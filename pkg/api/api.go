package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"news/pkg/postgres"

	"github.com/gorilla/mux"
)

// API приложения.
type API struct {
	R  *mux.Router     // маршрутизатор запросов
	db postgres.NewsDb // база данных
}

// Обертка для записи кода ответа (Response Status Code)
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// Переопределяем метод WriteHeader, чтобы запомнить статус для логов
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Конструктор API.
func New(db *postgres.NewsDb) *API {
	api := API{}
	api.db = *db
	api.R = mux.NewRouter()
	api.endpoints()
	return &api
}

// Router возвращает маршрутизатор запросов.
func (api *API) Router() *mux.Router {
	return api.R
}

// Регистрация методов API в маршрутизаторе запросов.
func (api *API) endpoints() {
	// Подключаем наш логгер ко всем запросам через Middleware
	api.R.Use(api.loggerMiddleware)

	// Единый эндпоинт для новостей (поиск + пагинация)
	api.R.HandleFunc("/news", api.getNews).Methods(http.MethodGet, http.MethodOptions)

	// Детальная новость
	api.R.HandleFunc("/news/{id:[0-9]+}", api.postByID).Methods(http.MethodGet, http.MethodOptions)

	// Статика
	api.R.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./webapp"))))
}

// Основной обработчик новостей
func (api *API) getNews(w http.ResponseWriter, r *http.Request) {
	sQuery := r.URL.Query().Get("s")     // Поиск
	pageStr := r.URL.Query().Get("page") // Страница

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	// Вызываем универсальный метод из Postgres, который мы написали ранее
	response, err := api.db.GetNews(sQuery, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Получаем пост по ID
func (api *API) postByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	post, err := api.db.PostByID(id)
	if err != nil {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(post)
}

// Middleware для Request ID и Логирования
func (api *API) loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Сквозной ID: получаем или генерируем
		reqID := r.URL.Query().Get("request_id")
		if len(reqID) < 6 {
			reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}

		// Записываем ID в контекст
		ctx := context.WithValue(r.Context(), "request_id", reqID)

		// Добавляем ID в заголовок ответа
		w.Header().Set("X-Request-ID", reqID)

		// 2. Подготовка к логам
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}

		// Передаем управление дальше
		next.ServeHTTP(rw, r.WithContext(ctx))

		// 3. Вывод лога (время, IP, метод, код, ID)
		log.Printf(
			"[%s] IP: %s | %s %s | STATUS: %d | ID: %s | DUR: %v",
			time.Now().Format("2006-01-02 15:04:05"),
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			rw.statusCode,
			reqID,
			time.Since(start),
		)
	})
}
