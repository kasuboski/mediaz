package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/kasuboski/mediaz/pkg/pagination"
)

// ParsePaginationParams extracts and validates pagination params from request
func ParsePaginationParams(r *http.Request) (pagination.Params, error) {
	params := pagination.Params{
		Page:     1,
		PageSize: 0,
	}

	qp := r.URL.Query()

	if pageStr := qp.Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return params, fmt.Errorf("invalid page parameter: must be positive integer")
		}
		params.Page = page
	}

	if pageSizeStr := qp.Get("pageSize"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 0 {
			return params, fmt.Errorf("invalid pageSize parameter: must be non-negative integer")
		}
		params.PageSize = pageSize
	}

	return params, nil
}
