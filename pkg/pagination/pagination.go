package pagination

type Params struct {
	Page     int
	PageSize int
}

func (p Params) CalculateOffsetLimit() (offset, limit int) {
	if p.PageSize <= 0 || p.Page <= 0 || p.Page-1 > int(^uint(0)>>1)/p.PageSize {
		return 0, 0
	}
	return (p.Page - 1) * p.PageSize, p.PageSize
}

func (p Params) BuildMeta(totalItems int) Meta {
	totalPages := 0
	if p.PageSize > 0 && totalItems > 0 {
		totalPages = 1 + (totalItems-1)/p.PageSize
	}
	return Meta{
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

// PageResult is the consistent HTTP catalog pagination envelope.
type PageResult[T any] struct {
	Items      []T  `json:"items"`
	Pagination Meta `json:"pagination"`
}

// Page slices an already ordered catalog while retaining accurate metadata.
func Page[T any](items []T, params Params) PageResult[T] {
	offset, limit := params.CalculateOffsetLimit()
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	pageItems := append([]T{}, items[offset:end]...)
	return PageResult[T]{Items: pageItems, Pagination: params.BuildMeta(len(items))}
}

type Meta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
