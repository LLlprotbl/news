package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"news/pkg/api"
	"news/pkg/postgres"

	"github.com/stretchr/testify/require"
)

// type mockRepository struct {
// }

// func (mockRepository) Posts(n int) ([]postgres.Post, error) {
// 	return []postgres.Post{}, nil
// }

func TestAPI_HeadersMiddleware(t *testing.T) {
	// Строка подключения к тестовой базе данных PostgreSQL
	connStr := "postgres://postgres:gfhjkm@172.23.252.228:5432/Newnews"

	// Создаем подключение к базе данных
	// NewNewsDb возвращает экземпляр для работы с данными
	newsDB, err := postgres.NewNewsDb(connStr)
	// Проверяем, что подключение прошло без ошибок
	// require.NoError автоматически завершит тест с ошибкой если err != nil
	require.NoError(t, err)
	// Отложенное закрытие подключения к БД при завершении функции
	defer newsDB.Close()

	// Создаем тестовый HTTP сервер
	// httptest.NewServer запускает локальный сервер на случайном порту
	// api.New(newsDB).Router() создает роутер API с подключением к БД
	srv := httptest.NewServer(api.New(newsDB).Router())
	// Гарантируем закрытие сервера после завершения теста
	defer srv.Close()
	// Выполняем HTTP GET запрос к тестовому серверу
	// Путь: /news/10 - вероятно, получение новости с ID=10
	rsp, err := http.Get(srv.URL + "/news/10")
	// Проверяем, что запрос выполнился без ошибок
	require.NoError(t, err)
	// Гарантируем закрытие тела ответа после чтения
	defer rsp.Body.Close()
	// Проверяем статус код ответа
	// Ожидаем 200 OK - успешный запрос
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	// Читаем все тело ответа в байтовый срез
	// io.ReadAll читает до EOF (конца файла/потока)
	b, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	// Логируем содержимое тела ответа для отладки
	t.Logf("body:%s", b)
	// Проверка заголовков ответа (основная цель middleware)
	contentType := rsp.Header.Get("Content-Type")
	require.Equal(t, "application/json", contentType)

	// var tPost []postgres.Post
	// err = json.NewDecoder(rsp.Body).Decode(&tPost)
	// require.NoError(t, err)
	// t.Logf("body:%#v", tPost) // структурированынй вывод в консоль
}
