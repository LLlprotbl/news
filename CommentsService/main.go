package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

type Comment struct {
	ID        int       `json:"id"`
	NewsID    int       `json:"news_id"`
	ParentID  *int      `json:"parent_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// MIDDLEWARE

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
		if reqID == "" {
			reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}

		next.ServeHTTP(rw, r)

		log.Printf("[%s] COMMENTS | %s %s | ID: %s | Статус: %d | Время: %v",
			time.Now().Format("15:04:05"), r.Method, r.URL.Path, reqID, rw.statusCode, time.Since(start))
	})
}

// РАБОТА С БД

func initDB(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS comments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        news_id INTEGER NOT NULL,
        parent_id INTEGER,
        author TEXT NOT NULL,
        text TEXT NOT NULL,
        created_at DATETIME NOT NULL
    );`
	_, err := db.Exec(query)
	return err
}

func main() {
	db, err := sql.Open("sqlite", "./comments.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/comments", commentsHandler(db))

	wrappedMux := loggerMiddleware(mux)

	fmt.Println("CommentsService запущен на :8081")
	log.Fatal(http.ListenAndServe(":8081", wrappedMux))
}

func commentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			addComment(w, r, db)
		case http.MethodGet:
			getComments(w, r, db)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func addComment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var c Comment
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(
		`INSERT INTO comments (news_id, parent_id, author, text, created_at)
         VALUES (?, ?, ?, ?, datetime('now'))`,
		c.NewsID, c.ParentID, c.Author, c.Text,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "ok", "id": id})
}

func getComments(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	newsID := r.URL.Query().Get("news_id")
	if newsID == "" {
		http.Error(w, "news_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(
		`SELECT id, news_id, parent_id, author, text, created_at 
         FROM comments 
         WHERE news_id = ? 
         ORDER BY created_at`,
		newsID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.NewsID, &c.ParentID, &c.Author, &c.Text, &c.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		comments = append(comments, c)
	}
	if comments == nil {
		comments = []Comment{}
	}
	writeJSON(w, comments)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
