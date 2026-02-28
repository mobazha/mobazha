package api

import (
	"net/http"
	"strconv"
)

const maxPageSize = 100

func intQueryParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	if key == "pageSize" && v > maxPageSize {
		return maxPageSize
	}
	return v
}
