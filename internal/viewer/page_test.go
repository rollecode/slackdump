package viewer

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/rusq/slack"
)

func testMessages(n int) []slack.Message {
	mm := make([]slack.Message, n)
	for i := range mm {
		mm[i].Timestamp = fmt.Sprintf("%d.000100", 1600000000+i)
	}
	return mm
}

func tsOf(i int) string { return strconv.Itoa(1600000000+i) + ".000100" }

func TestPaginate(t *testing.T) {
	const total = 1000
	tests := []struct {
		name         string
		q            pageQuery
		first, last  int
		older, newer string
		prev         int // -1 when there is no previous message
	}{
		{"latest", pageQuery{}, total - pageSize, total - 1, tsOf(total - pageSize), "", total - pageSize - 1},
		{"oldest", pageQuery{start: startOldest}, 0, pageSize - 1, "", tsOf(pageSize - 1), -1},
		{"before", pageQuery{startBefore, tsOf(500)}, 500 - pageSize, 499, tsOf(500 - pageSize), "", 500 - pageSize - 1},
		{"before start", pageQuery{startBefore, tsOf(10)}, 0, 9, "", "", -1},
		{"after", pageQuery{startAfter, tsOf(500)}, 501, 500 + pageSize, "", tsOf(500 + pageSize), 500},
		{"after end", pageQuery{startAfter, tsOf(990)}, 991, 999, "", "", 990},
		{"from", pageQuery{startFrom, tsOf(500)}, 500, 500 + pageSize - 1, tsOf(500), tsOf(500 + pageSize - 1), 499},
		{"at", pageQuery{startAt, tsOf(500)}, 500 - pageSize/2, 500 + pageSize/2 - 1, tsOf(500 - pageSize/2), tsOf(500 + pageSize/2 - 1), 500 - pageSize/2 - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg, err := paginate(seqOf(testMessages(total)), tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(pg.msgs); got != tt.last-tt.first+1 {
				t.Fatalf("got %d messages, want %d", got, tt.last-tt.first+1)
			}
			if pg.msgs[0].Timestamp != tsOf(tt.first) || pg.msgs[len(pg.msgs)-1].Timestamp != tsOf(tt.last) {
				t.Errorf("window %s..%s, want %s..%s", pg.msgs[0].Timestamp, pg.msgs[len(pg.msgs)-1].Timestamp, tsOf(tt.first), tsOf(tt.last))
			}
			if pg.older != tt.older || pg.newer != tt.newer {
				t.Errorf("older/newer = %q/%q, want %q/%q", pg.older, pg.newer, tt.older, tt.newer)
			}
			if tt.prev < 0 {
				if pg.prev != "" {
					t.Errorf("prev = %v, want none", pg.prev)
				}
			} else if m, ok := pg.prev.(slack.Message); !ok || m.Timestamp != tsOf(tt.prev) {
				t.Errorf("prev = %v, want %s", pg.prev, tsOf(tt.prev))
			}
		})
	}
}

func TestPaginateYears(t *testing.T) {
	ts := func(y int, m time.Month) string {
		return strconv.FormatInt(time.Date(y, m, 15, 12, 0, 0, 0, time.Local).Unix(), 10) + ".000100"
	}
	mm := []slack.Message{{}, {}, {}, {}}
	mm[0].Timestamp = ts(2015, time.June)
	mm[1].Timestamp = ts(2015, time.June)
	mm[2].Timestamp = ts(2015, time.August)
	mm[3].Timestamp = ts(2017, time.January)
	pg, err := paginate(seqOf(mm), pageQuery{start: startOldest})
	if err != nil {
		t.Fatal(err)
	}
	want := []yearMark{
		{2015, []monthMark{{"Jun", "2015-06", ts(2015, time.June)}, {"Aug", "2015-08", ts(2015, time.August)}}},
		{2017, []monthMark{{"Jan", "2017-01", ts(2017, time.January)}}},
	}
	if !reflect.DeepEqual(pg.years, want) {
		t.Errorf("years = %v, want %v", pg.years, want)
	}
}
