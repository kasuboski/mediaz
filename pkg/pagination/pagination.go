package pagination

type Params struct {
	Page     int
	PageSize int
}

func (p Params) CalculateOffsetLimit() (offset, limit int) {
	if p.PageSize == 0 {
		return 0, 0
	}
	offset = (p.Page - 1) * p.PageSize
	limit = p.PageSize
	return offset, limit
}

func (p Params) BuildMeta(totalItems int) Meta {
	totalPages := 0
	if p.PageSize > 0 {
		totalPages = (totalItems + p.PageSize - 1) / p.PageSize
	}
	return Meta{
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

type Meta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
