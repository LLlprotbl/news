package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"news/pkg/api"
	"news/pkg/postgres"
	"news/pkg/rss"
)

func main() {
	// Чтение конфигурации RSS
	configFile, err := os.Open("config.json")
	if err != nil {
		log.Fatal("Open config:", err)
	}
	defer configFile.Close()

	var rssConfig rss.Config
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&rssConfig)
	if err != nil {
		log.Fatal("Decode config:", err)
	}

	// Создание хранилища
	connStr := "postgres://postgres:gfhjkm@172.23.252.228:5432/Newnews"

	newsDB, err := postgres.NewNewsDb(connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer newsDB.Close()
	// Создание парсера RSS
	parser := rss.NewParser(rssConfig)

	// Каналы для обмена данными
	postsChan := make(chan []rss.Item)
	errChan := make(chan error)

	// Запуск парсера в отдельной горутине
	go parser.Start(postsChan, errChan)

	// Обработка полученных постов
	go func() {
		for items := range postsChan {
			var posts []postgres.Post
			for _, item := range items {
				pubTime, err := time.Parse(time.RFC1123Z, item.PubDate)
				if err != nil {
					// Пробуем другие форматы даты
					pubTime, err = time.Parse(time.RFC1123, item.PubDate)
					if err != nil {
						pubTime = time.Now()
					}
				}

				posts = append(posts, postgres.Post{
					Title:   item.Title,
					Content: item.Сontent,
					PubTime: pubTime.Unix(),
					Link:    item.Link,
				})
			}

			if len(posts) > 0 {
				err := newsDB.AddPosts(posts)
				if err != nil {
					log.Printf("Add posts error: %v", err)
				} else {
					log.Printf("Added %d posts", len(posts))
				}
			}
		}
	}()

	// Обработка ошибок
	go func() {
		for err := range errChan {
			log.Printf("RSS error: %v", err)
		}
	}()

	// Создание API

	// Запуск сервера
	log.Println("Server starting on :80")
	err = http.ListenAndServe(":80", api.New(newsDB).Router())
	if err != nil {
		log.Fatal(err)
	}
}
