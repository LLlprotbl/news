package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"news/pkg/postgres"

	"github.com/gorilla/mux"
)

// API приложения.
type API struct {
	R  *mux.Router     // маршрутизатор запросов
	db postgres.NewsDb // база данных
}

// Конструктор API.
func New(db *postgres.NewsDb) *API {
	api := API{}
	api.db = *db
	api.R = mux.NewRouter()
	api.endpoints()
	return &api
}

// HeadersMiddleware устанавливает заголовки ответа сервера.
func (api *API) HeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Router возвращает маршрутизатор запросов.
func (api *API) Router() *mux.Router {
	return api.R
}

// Регистрация методов API в маршрутизаторе запросов.
func (api *API) endpoints() {
	// получить n последних новостей
	api.R.HandleFunc("/news/{n}", api.posts).Methods(http.MethodGet, http.MethodOptions)
	// веб-приложение
	api.R.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./webapp"))))
}

// posts получаем посты
func (api *API) posts(w http.ResponseWriter, r *http.Request) {
	s := mux.Vars(r)["n"]
	n, err := strconv.Atoi(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	posts, err := api.db.Posts(n)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Кодируем результат в JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}
