package services

const DefaultPagingLimit = 10

type Pagination[T any] struct {
	Items      []T
	NextOffset int
	PrevOffset int
	NextPage   int
	PrevPage   int
	Page       int
}

func NewPagination[T any](offset int, items []T) Pagination[T] {
	var pagination Pagination[T]
	pagination.Items = items[:min(len(items), DefaultPagingLimit)]
	pagination.Page = 1 + offset/DefaultPagingLimit

	if len(items) > DefaultPagingLimit {
		pagination.NextOffset = offset + DefaultPagingLimit
		pagination.NextPage = 1 + pagination.NextOffset/DefaultPagingLimit
	} else {
		pagination.NextOffset = -1
	}

	if offset > 0 {
		pagination.PrevOffset = max(offset-DefaultPagingLimit, 0)
		pagination.PrevPage = 1 + pagination.PrevOffset/DefaultPagingLimit
	} else {
		pagination.PrevOffset = -1
	}

	return pagination
}
