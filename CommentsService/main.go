package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	Approved  bool      `json:"approved"`
}

var moderationChan = make(chan int, 100) // Канал модерации

func initDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		news_id INTEGER NOT NULL,
		parent_id INTEGER,
		author TEXT NOT NULL,
		text TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		approved INTEGER DEFAULT 0
	);
	`
	_, err := db.Exec(query)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func main() {
	db, err := sql.Open("sqlite", "./comments.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	if err := initDB(db); err != nil {
		log.Fatal(err)
	}

	// Горутина-модератор
	go func(db *sql.DB) {
		badWords := []string{"qwerty", "йцукен", "zxvbnm"}

		for commentID := range moderationChan {
			var text string

			err := db.QueryRow(
				"SELECT text FROM comments WHERE id = ?",
				commentID,
			).Scan(&text)
			if err != nil {
				log.Println("moderation error:", err)
				continue
			}

			approved := true
			for _, w := range badWords {
				if strings.Contains(strings.ToLower(text), w) {
					approved = false
					break
				}
			}

			_, err = db.Exec(
				"UPDATE comments SET approved = ? WHERE id = ?",
				boolToInt(approved), commentID,
			)
			if err != nil {
				log.Println("update error:", err)
			}
			log.Println("comment", commentID, "approved =", approved)
		}
	}(db)

	fmt.Println("SQLite подключена, таблица готова")

	mux := http.NewServeMux()
	mux.HandleFunc("/comments", commentsHandler(db))

	log.Println("CommentsService запущен на :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
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

// POST /comments — добавление комментария
func addComment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var c Comment

	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(
		`INSERT INTO comments (news_id, parent_id, author, text, created_at)
		 VALUES (?, ?, ?,?, datetime('now'))`,
		c.NewsID, c.ParentID, c.Author, c.Text,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		http.Error(w, "cannot get id", http.StatusInternalServerError)
		return
	}

	// отправляем в модерацию
	moderationChan <- int(id)

	writeJSON(w, map[string]any{
		"status": "ok",
		"id":     id,
	})
}

// GET /comments?news_id=1 — получение комментариев
func getComments(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	newsID := r.URL.Query().Get("news_id")
	if newsID == "" {
		http.Error(w, "news_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(
		`SELECT id, news_id, parent_id, author, text, created_at, approved
FROM comments
WHERE news_id = ? AND approved = 1
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
		if err := rows.Scan(
			&c.ID, &c.NewsID, &c.ParentID, &c.Author, &c.Text, &c.CreatedAt, &c.Approved,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		comments = append(comments, c)
	}

	writeJSON(w, comments)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
