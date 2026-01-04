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

// Структуры
// Структура для пагинации
type Pagination struct {
	TotalPages   int `json:"total_pages"`
	CurrentPage  int `json:"current_page"`
	ItemsPerPage int `json:"items_per_page"`
}

type NewsResponse struct {
	News       []Post     `json:"news"`
	Pagination Pagination `json:"pagination"`
}

// Структура для БД
type Post struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	PubTime int64  `json:"pub_time"`
	Link    string `json:"link"`
}

// Универсальный метод: Поиск + Пагинация
func (s *NewsDb) GetNews(search string, page int) (NewsResponse, error) {
	const itemsPerPage = 15 // Фиксировано по ТЗ
	offset := (page - 1) * itemsPerPage

	// 1. Сначала считаем общее количество новостей, подходящих под поиск
	// Это нужно, чтобы вычислить количество страниц (TotalPages)
	var totalItems int
	countQuery := "SELECT count(*) FROM posts WHERE title ILIKE $1 OR content ILIKE $1"
	err := s.Db.QueryRow(context.Background(), countQuery, "%"+search+"%").Scan(&totalItems)
	if err != nil {
		return NewsResponse{}, fmt.Errorf("ошибка подсчета строк: %w", err)
	}

	// Считаем общее кол-во страниц.
	// Формула (total + limit - 1) / limit — это округление вверх
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage
	if totalPages == 0 && totalItems == 0 {
		totalPages = 1
	}

	// 2. Получаем сами новости с использованием LIMIT (сколько взять) и OFFSET (сколько пропустить)
	rows, err := s.Db.Query(context.Background(), `
		SELECT id, title, content, pub_time, link 
		FROM posts 
		WHERE title ILIKE $1 OR content ILIKE $1
		ORDER BY pub_time DESC 
		LIMIT $2 OFFSET $3`,
		"%"+search+"%", itemsPerPage, offset)
	if err != nil {
		return NewsResponse{}, fmt.Errorf("ошибка получения данных: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var pubTime time.Time
		err := rows.Scan(&p.ID, &p.Title, &p.Content, &pubTime, &p.Link)
		if err != nil {
			return NewsResponse{}, err
		}
		p.PubTime = pubTime.Unix()
		posts = append(posts, p)
	}

	// Если новостей нет, возвращаем пустой массив, а не nil (чтобы в JSON было [])
	if posts == nil {
		posts = []Post{}
	}

	return NewsResponse{
		News: posts,
		Pagination: Pagination{
			TotalPages:   totalPages,
			CurrentPage:  page,
			ItemsPerPage: itemsPerPage,
		},
	}, nil
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

// // Posts возвращает список постов
// func (s *NewsDb) Posts(n int) ([]Post, error) {
// 	rows, err := s.Db.Query(context.Background(), `
//         SELECT id, title, content, pub_time, link
//         FROM posts
//         ORDER BY pub_time DESC
//         LIMIT $1
//     `, n,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var posts []Post
// 	for rows.Next() {
// 		var t Post
// 		var pubTime time.Time

// 		err = rows.Scan(
// 			&t.ID,
// 			&t.Title,
// 			&t.Content,
// 			&pubTime,
// 			&t.Link,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}

// 		t.PubTime = pubTime.Unix()

// 		posts = append(posts, t)
// 	}
// 	return posts, rows.Err()
// }

// PostByID возвращает одну новость по её ID
func (s *NewsDb) PostByID(id int) (Post, error) {
	var p Post
	var pubTime time.Time
	err := s.Db.QueryRow(context.Background(), `
    SELECT id, title, content, pub_time, link
    FROM posts
    WHERE id = $1
`, id).Scan(&p.ID, &p.Title, &p.Content, &pubTime, &p.Link)
	if err != nil {
		return p, err
	}
	p.PubTime = pubTime.Unix()
	return p, nil
}

// func (s *NewsDb) SearchPosts(query string) ([]Post, error) {
// 	rows, err := s.Db.Query(context.Background(), `
// 		SELECT id, title, content, pub_time, link
// 		FROM posts
// 		WHERE title ILIKE $1 OR content ILIKE $1
// 		ORDER BY pub_time DESC
// 		LIMIT 10`,
// 		"%"+query+"%") // % — это маска для поиска подстроки
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var posts []Post
// 	for rows.Next() {
// 		var p Post
// 		var pubTime time.Time
// 		err := rows.Scan(&p.ID, &p.Title, &p.Content, &pubTime, &p.Link)
// 		if err != nil {
// 			return nil, err
// 		}
// 		p.PubTime = pubTime.Unix()
// 		posts = append(posts, p)
// 	}
// 	return posts, nil
// }

// Close закрывает соединение с БД
func (s *NewsDb) Close() {
	s.Db.Close()
}
