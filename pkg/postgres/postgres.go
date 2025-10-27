package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type NewsDb struct {
	Db *pgxpool.Pool
}

// NewNewsDb создает новый экземпляр NewsDb и устанавливает соединение с БД
// connectionString - строка подключения к PostgreSQL

func NewNewsDb(connectionString string) (*NewsDb, error) {
	// Создаем пул соединений с базой данных
	db, err := pgxpool.Connect(context.Background(), connectionString)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	// Создаем и возвращаем наш объект
	newsDB := NewsDb{
		Db: db,
	}
	return &newsDB, nil
}

// Структура для БД
type Post struct {
	ID      int    // номер записи
	Title   string // заголовок публикации
	Content string // содержание публикации
	PubTime int64  // время публикации
	Link    string // ссылка на источник
}

// AddPosts добавляет новые посты в БД
func (s *NewsDb) AddPosts(adPosts []Post) error {
	ctx := context.Background()

	for _, post := range adPosts {
		_, err := s.Db.Exec(ctx, `
            INSERT INTO posts (title, content, pub_time, link)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (link) DO NOTHING
        `, post.Title, post.Content, time.Unix(post.PubTime, 0), post.Link)
		if err != nil {
			return fmt.Errorf("insert post: %w", err)
		}
	}
	return nil
}

// Posts возвращает список постов
func (s *NewsDb) Posts(n int) ([]Post, error) {
	rows, err := s.Db.Query(context.Background(), `
        SELECT 
            id,
            title, 
            content,
            pub_time,
            link
        FROM posts
        ORDER BY pub_time DESC
        LIMIT $1
    `, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var t Post
		var pubTime time.Time

		err = rows.Scan(
			&t.ID,
			&t.Title,
			&t.Content,
			&pubTime,
			&t.Link,
		)
		if err != nil {
			return nil, err
		}

		t.PubTime = pubTime.Unix()

		posts = append(posts, t)
	}
	return posts, rows.Err()
}

// Close закрывает соединение с БД
func (s *NewsDb) Close() {
	s.Db.Close()
}
