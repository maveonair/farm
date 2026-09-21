package server

import (
	"errors"
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

type paginationResponse struct {
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasNext bool `json:"has_next"`
}

type pageQuery struct {
	number  int
	perPage int
	offset  int
}

func (q pageQuery) limit() int {
	return q.perPage + 1
}

func parsePage(r *http.Request) (pageQuery, error) {
	page, err := positiveInt(r.URL.Query().Get("page"), 1)
	if err != nil {
		return pageQuery{}, errors.New("invalid page")
	}
	perPage, err := positiveInt(r.URL.Query().Get("per_page"), defaultPageSize)
	if err != nil || perPage > maxPageSize {
		return pageQuery{}, errors.New("invalid per_page")
	}

	const maxInt = int(^uint(0) >> 1)
	if page-1 > maxInt/perPage {
		return pageQuery{}, errors.New("invalid page")
	}

	return pageQuery{number: page, perPage: perPage, offset: (page - 1) * perPage}, nil
}

func positiveInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsed, nil
}

func finishPage[T any](items []T, query pageQuery) ([]T, paginationResponse) {
	hasNext := len(items) > query.perPage
	if hasNext {
		items = items[:query.perPage]
	}
	return items, paginationResponse{Page: query.number, PerPage: query.perPage, HasNext: hasNext}
}
