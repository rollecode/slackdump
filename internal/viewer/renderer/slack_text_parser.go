package renderer

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	emj "github.com/enescakir/emoji"
	"github.com/rusq/slack"

	st "github.com/rusq/slackdump/v4/internal/structures"
)

var (
	reToken      = regexp.MustCompile(`<([^<>\n]+)>`)
	reCodeBlock  = regexp.MustCompile("(?s)```\\s*(.*?)```")
	reInlineCode = regexp.MustCompile("`([^`\n]+)`")
	reBold       = regexp.MustCompile(`(^|[\s(>])\*([^*\n]+?)\*($|[\s.,!?;:)<])`)
	reItalic     = regexp.MustCompile(`(^|[\s(>])_([^_\n]+?)_($|[\s.,!?;:)<])`)
	reStrike     = regexp.MustCompile(`(^|[\s(>])~([^~\n]+?)~($|[\s.,!?;:)<])`)
	reEmoji      = regexp.MustCompile(`:([a-z0-9_+\-']+):`)
	rePlacehold  = regexp.MustCompile("\x00(\\d+)\x00")
)

// mrkdwn converts Slack message text into HTML.
func (s *Slack) mrkdwn(text string) string {
	var held []string
	hold := func(h string) string {
		held = append(held, h)
		return fmt.Sprintf("\x00%d\x00", len(held)-1)
	}

	// Slack escapes only &, < and >; unescape so every piece is escaped once.
	text = reCodeBlock.ReplaceAllStringFunc(text, func(m string) string {
		body := reCodeBlock.FindStringSubmatch(m)[1]
		return hold(`<pre class="code-block">` + html.EscapeString(html.UnescapeString(body)) + `</pre>`)
	})
	text = reInlineCode.ReplaceAllStringFunc(text, func(m string) string {
		body := reInlineCode.FindStringSubmatch(m)[1]
		return hold(`<code>` + html.EscapeString(html.UnescapeString(body)) + `</code>`)
	})
	text = reToken.ReplaceAllStringFunc(text, func(m string) string {
		return hold(s.token(reToken.FindStringSubmatch(m)[1]))
	})

	text = html.EscapeString(html.UnescapeString(text))
	text = reEmoji.ReplaceAllStringFunc(text, func(m string) string {
		if e := emj.Parse(m); e != m {
			return e
		}
		return m
	})
	for range 2 { // second pass catches adjacent matches sharing a delimiter
		text = reBold.ReplaceAllString(text, "$1<b>$2</b>$3")
		text = reItalic.ReplaceAllString(text, "$1<i>$2</i>$3")
		text = reStrike.ReplaceAllString(text, "$1<s>$2</s>$3")
	}
	text = quoteLines(text)
	text = strings.ReplaceAll(text, "\n", "<br>")

	for strings.Contains(text, "\x00") {
		text = rePlacehold.ReplaceAllStringFunc(text, func(m string) string {
			var i int
			fmt.Sscanf(rePlacehold.FindStringSubmatch(m)[1], "%d", &i)
			return held[i]
		})
	}
	// Block elements bring their own spacing; drop the line breaks around them.
	return blockBreaks.Replace(text)
}

var blockBreaks = strings.NewReplacer(
	"<br><pre", "<pre", "</pre><br>", "</pre>",
	"<br><blockquote", "<blockquote", "</blockquote><br>", "</blockquote>",
)

// quoteLines wraps runs of lines starting with "&gt; " in a blockquote.
func quoteLines(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	var quote []string
	flush := func() {
		if len(quote) == 0 {
			return
		}
		out = append(out, `<blockquote class="quote">`+strings.Join(quote, "<br>")+`</blockquote>`)
		quote = nil
	}
	for _, l := range lines {
		if rest, ok := strings.CutPrefix(l, "&gt;"); ok {
			quote = append(quote, strings.TrimPrefix(rest, " "))
			continue
		}
		flush()
		out = append(out, l)
	}
	flush()
	return strings.Join(out, "\n")
}

// token renders the inside of a <...> Slack token.
func (s *Slack) token(tok string) string {
	ref, label, _ := strings.Cut(tok, "|")
	label = html.UnescapeString(label)
	switch {
	case strings.HasPrefix(ref, "@"):
		id := ref[1:]
		return `<span class="mention">@` + html.EscapeString(st.NVL(s.userName(id), label, id)) + `</span>`
	case strings.HasPrefix(ref, "#"):
		id := ref[1:]
		name := st.NVL(label, id)
		if c, ok := s.cc[id]; ok {
			name = st.NVL(c.Name, name)
		}
		h := `<span class="mention">#` + html.EscapeString(name) + `</span>`
		if s.routes != nil {
			h = `<a class="channel-link" href="` + html.EscapeString(s.routes.Channel(id)) + `">` + h + `</a>`
		}
		return h
	case strings.HasPrefix(ref, "!subteam^"):
		return `<span class="mention">` + html.EscapeString(st.NVL(label, "@group")) + `</span>`
	case strings.HasPrefix(ref, "!date^"):
		return html.EscapeString(label)
	case strings.HasPrefix(ref, "!"):
		return `<span class="mention broadcast">@` + html.EscapeString(ref[1:]) + `</span>`
	}
	href := html.UnescapeString(ref)
	if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "mailto:") {
		return html.EscapeString("<" + html.UnescapeString(tok) + ">")
	}
	return `<a href="` + html.EscapeString(href) + `" target="_blank" rel="noopener noreferrer">` +
		html.EscapeString(st.NVL(label, strings.TrimPrefix(href, "mailto:"))) + `</a>`
}

func (s *Slack) userName(id string) string {
	u, ok := s.uu[id]
	if !ok {
		return ""
	}
	return userDisplayName(&u)
}

// userDisplayName picks the name Slack itself shows next to a message.
func userDisplayName(u *slack.User) string {
	return st.NVL(u.Profile.DisplayName, u.Profile.RealName, u.RealName, u.Name)
}
