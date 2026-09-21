package store

const (
	defaultListLimit = 50
	maxListLimit     = 101 // Maximum API page plus its lookahead row.
)

func listLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func listOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
