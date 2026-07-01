package repository

import (
	"math"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestLimitOffsetRejectsOutOfRangeValues(t *testing.T) {
	cases := []service.PageInput{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: math.MaxInt32 + 1},
		{Page: 214748366, PageSize: 10},
	}
	for _, page := range cases {
		if _, _, err := limitOffset(page); err == nil {
			t.Fatalf("limitOffset(%+v) error = nil, want validation error", page)
		}
	}
	limit, offset, err := limitOffset(service.PageInput{Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if limit != 10 || offset != 10 {
		t.Fatalf("limitOffset = %d, %d; want 10, 10", limit, offset)
	}
}
