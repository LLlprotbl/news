package rss

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RSS структуры для парсинга
type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Сontent string `xml:"content"`
	Items   []Item `xml:"item"`
}

type Item struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Сontent string `xml:"description"`
	PubDate string `xml:"pubDate"`
	Guid    string `xml:"guid"`
}

// Config конфигурация RSS
type Config struct {
	URLs          []string      `json:"rss"`
	RequestPeriod time.Duration `json:"request_period"`
}

// Parser для работы с RSS
type Parser struct {
	config Config
}

func NewParser(config Config) *Parser {
	return &Parser{
		config: config,
	}
}

// ParseFeed парсит RSS по URL и возвращает результат через каналы
func (p *Parser) ParseFeed(url string, postsChan chan<- []Item, errChan chan<- error) {
	items, err := p.parseURL(url)
	if err != nil {
		errChan <- fmt.Errorf("feed %s: %w", url, err)
		return
	}

	postsChan <- items
}

// parseURL выполняет HTTP запрос и парсит RSS
func (p *Parser) parseURL(url string) ([]Item, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return nil, err
	}

	return rss.Channel.Items, nil
}

// Start запускает периодический парсинг RSS лент
func (p *Parser) Start(postsChan chan<- []Item, errChan chan<- error) {
	ticker := time.NewTicker(p.config.RequestPeriod * time.Minute)
	defer ticker.Stop()

	// Первоначальный парсинг
	p.parseAllFeeds(postsChan, errChan)

	for {
		select {
		case <-ticker.C:
			p.parseAllFeeds(postsChan, errChan)
		}
	}
}

// parseAllFeeds парсит все RSS ленты
func (p *Parser) parseAllFeeds(postsChan chan<- []Item, errChan chan<- error) {
	for _, url := range p.config.URLs {
		go p.ParseFeed(url, postsChan, errChan)
	}
}
