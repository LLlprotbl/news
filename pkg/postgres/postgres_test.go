package postgres_test

import (
	"context"
	"testing"
	"time"

	"news/pkg/postgres"
)

func TestNewsDb(t *testing.T) {
	connStr := "postgres://postgres:gfhjkm@172.23.252.228:5432/testNews"

	newsDB, err := postgres.NewNewsDb(connStr)
	if err != nil {
		t.Fatalf("не удалось подключиться: %v", err)
	}
	defer newsDB.Db.Close()

	if newsDB.Db == nil {
		t.Fatal("ожидали непустое поле db, получили nil")
	}

	var result int
	err = newsDB.Db.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("ошибка при запросе: %v", err)
	}
	if result != 1 {
		t.Fatalf("ожидали результат 1, получили %d", result)
	}

	// --- Тест на добавление постов ---
	testPosts := []postgres.Post{
		{
			Title:   "Test Post 1",
			Content: "Content of test post 1",
			PubTime: time.Now().Unix(),
			Link:    "https://example.com/post1",
		},
		{
			Title:   "Test Post 2",
			Content: "Content of test post 2",
			PubTime: time.Now().Unix(),
			Link:    "https://example.com/post2",
		},
	}

	err = newsDB.AddPosts(testPosts)
	if err != nil {
		t.Fatalf("ошибка при добавлении постов: %v", err)
	}

	// --- Тест на получение постов ---
	posts, err := newsDB.Posts(10)
	if err != nil {
		t.Fatalf("ошибка при получении постов: %v", err)
	}

	// Проверяем, что хотя бы два тестовых поста вернулись
	found1, found2 := false, false
	for _, p := range posts {
		if p.Link == "https://example.com/post1" {
			found1 = true
		}
		if p.Link == "https://example.com/post2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("тестовые посты не найдены в базе")
	}
}
