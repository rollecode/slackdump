package renderer

import (
	"testing"

	"github.com/rusq/slack"
)

func TestSlack_mrkdwn(t *testing.T) {
	s := &Slack{
		uu: map[string]slack.User{"U1": {ID: "U1", Name: "jdoe", Profile: slack.UserProfile{DisplayName: "Jane"}}},
		cc: map[string]slack.Channel{"C1": {GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C1"}, Name: "general"}}},
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain escaped", "a &lt;b&gt; &amp; c", "a &lt;b&gt; &amp; c"},
		{"user mention", "hi <@U1>", `hi <span class="mention">@Jane</span>`},
		{"unknown user uses label", "<@U9|bob>", `<span class="mention">@bob</span>`},
		{"channel", "<#C1>", `<span class="mention">#general</span>`},
		{"broadcast", "<!here>", `<span class="mention broadcast">@here</span>`},
		{"link with label", "<https://x.test/a_b_c|site>", `<a href="https://x.test/a_b_c" target="_blank" rel="noopener noreferrer">site</a>`},
		{"bold italic strike", "*b* _i_ ~s~", "<b>b</b> <i>i</i> <s>s</s>"},
		{"snake_case untouched", "my_var_name", "my_var_name"},
		{"inline code keeps stars", "`*x*`", "<code>*x*</code>"},
		{"code block", "see:\n```a < b```\nok", `see:<pre class="code-block">a &lt; b</pre>ok`},
		{"quote", "&gt; quoted\nnext", `<blockquote class="quote">quoted</blockquote>next`},
		{"emoji", ":beer: time", "\U0001F37A time"},
		{"script stays text", "<script>", "&lt;script&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.mrkdwn(tt.in); got != tt.want {
				t.Errorf("mrkdwn(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}
