package store

import "testing"

func TestListLimit(t *testing.T) {
	const requested = 25

	for _, test := range []struct {
		name  string
		limit int
		want  int
	}{
		{name: "default", want: defaultListLimit},
		{name: "requested", limit: requested, want: requested},
		{name: "maximum", limit: maxListLimit + 1, want: maxListLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := listLimit(test.limit); got != test.want {
				t.Fatalf("listLimit(%d) = %d, want %d", test.limit, got, test.want)
			}
		})
	}
}
