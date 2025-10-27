package api

import "news/pkg/postgres"

type repository interface {
	Posts(n int) ([]postgres.Post, error)
}
