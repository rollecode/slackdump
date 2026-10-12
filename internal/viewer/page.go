package viewer

import (
	"iter"
	"net/http"
	"slices"

	"github.com/rusq/slack"

	st "github.com/rusq/slackdump/v4/internal/structures"
)

// pageSize is how many channel messages the live viewer renders at once.
const pageSize = 300

type pageStart int

const (
	startLatest pageStart = iota
	startOldest
	startBefore // messages older than ts
	startAfter  // messages newer than ts
	startAt     // messages around ts
)

// startCookie holds the "open channels at" preference so the server can
// honour it on the first request.
const startCookie = "slackdump-viewer-start"

type pageQuery struct {
	start pageStart
	ts    string
}

// pageFromRequest reads the page position from the query string, falling
// back to the start preference cookie.
func pageFromRequest(r *http.Request) pageQuery {
	q := r.URL.Query()
	for _, p := range []struct {
		key   string
		start pageStart
	}{{"before", startBefore}, {"after", startAfter}, {"at", startAt}} {
		if ts := q.Get(p.key); ts != "" {
			return pageQuery{start: p.start, ts: ts}
		}
	}
	start := q.Get("start")
	if start == "" {
		if c, err := r.Cookie(startCookie); err == nil {
			start = c.Value
		}
	}
	if start == "oldest" {
		return pageQuery{start: startOldest}
	}
	return pageQuery{start: startLatest}
}

func (q pageQuery) isChunk() bool {
	return q.start == startBefore || q.start == startAfter
}

// messagePage is one window of a channel's messages.
type messagePage struct {
	msgs  []slack.Message
	prev  any    // message right before msgs, for day dividers and grouping
	older string // ts to load older messages from, empty if none
	newer string // ts to load newer messages from, empty if none
}

// paginate picks the window q asks for from it, which runs oldest first.
// ponytail: scans the whole channel per request; add a paged source query if
// large channels get slow.
func paginate(it iter.Seq2[slack.Message, error], q pageQuery) (messagePage, error) {
	pivot, err := parseTS(q.ts)
	if err != nil {
		return messagePage{}, err
	}
	beforeCap, afterCap := pageSize, 0
	switch q.start {
	case startOldest, startAfter:
		beforeCap, afterCap = 0, pageSize
	case startAt:
		beforeCap, afterCap = pageSize/2, pageSize/2
	}
	isBefore := func(ts int64) bool {
		switch q.start {
		case startLatest:
			return true
		case startOldest:
			return false
		case startAfter:
			return ts <= pivot
		}
		return ts < pivot
	}

	var (
		pg         = messagePage{prev: ""}
		head, tail []slack.Message
		dropped    bool
		moreAfter  bool
	)
	for m, err := range it {
		if err != nil {
			return messagePage{}, err
		}
		ts, _ := parseTS(m.Timestamp) // a bad timestamp sorts first instead of failing the page
		if isBefore(ts) {
			head = append(head, m)
			if len(head) > beforeCap {
				pg.prev, head, dropped = head[0], head[1:], true
			}
			continue
		}
		if len(tail) == afterCap {
			moreAfter = true
			break
		}
		tail = append(tail, m)
	}
	pg.msgs = slices.Concat(head, tail)
	if len(pg.msgs) == 0 {
		return pg, nil
	}
	if dropped {
		pg.older = pg.msgs[0].Timestamp
	}
	if moreAfter {
		pg.newer = pg.msgs[len(pg.msgs)-1].Timestamp
	}
	// A chunk is attached to messages already on the page, so it only
	// continues in the direction it was loaded.
	switch q.start {
	case startBefore:
		pg.newer = ""
	case startAfter:
		pg.older = ""
	}
	return pg, nil
}

func parseTS(ts string) (int64, error) {
	if ts == "" {
		return 0, nil
	}
	t, err := st.ParseSlackTS(ts)
	if err != nil {
		return 0, err
	}
	return t.UnixMicro(), nil
}

func seqOf(mm []slack.Message) iter.Seq2[slack.Message, error] {
	return func(yield func(slack.Message, error) bool) {
		for _, m := range mm {
			if !yield(m, nil) {
				return
			}
		}
	}
}
