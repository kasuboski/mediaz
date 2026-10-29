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
		PageSize: 50,
	}

	qp := r.URL.Query()

	if values, ok := qp["page"]; ok {
		pageStr := values[0]
		page, err := strconv.Atoi(pageStr)
		if len(values) != 1 || err != nil || page < 1 {
			return params, fmt.Errorf("invalid page parameter: must be positive integer")
		}
		params.Page = page
	}

	if values, ok := qp["pageSize"]; ok {
		pageSize, err := strconv.Atoi(values[0])
		if len(values) != 1 || err != nil || pageSize < 1 || pageSize > 100 {
			return params, fmt.Errorf("invalid pageSize parameter: must be an integer from 1 to 100")
		}
		params.PageSize = pageSize
	}
	if params.Page-1 > int(^uint(0)>>1)/params.PageSize {
		return params, fmt.Errorf("page and pageSize overflow offset")
	}

	return params, nil
}
