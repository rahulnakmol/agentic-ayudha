package hosts

import "testing"

func TestRenderAgentsMD(t *testing.T) {
	skills := AgentsSection{ID: "skills", Body: "## Skills\n\nnew skills text\n"}
	hostsSec := AgentsSection{ID: "hosts", Body: "## Hosts\n\nhosts text"}

	t.Run("empty input", func(t *testing.T) {
		got := string(RenderAgentsMD(nil, skills, hostsSec))
		want := "# AGENTS.md\n\n<!-- BEGIN aayudha:skills -->\n## Skills\n\nnew skills text\n<!-- END aayudha:skills -->\n\n" +
			"<!-- BEGIN aayudha:hosts -->\n## Hosts\n\nhosts text\n<!-- END aayudha:hosts -->\n"
		if got != want {
			t.Fatalf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("replace in place, append new, keep the product's text", func(t *testing.T) {
		existing := "# Product agents\n\nOur rules.\n\n<!-- BEGIN aayudha:skills -->\n## Skills\n\nold text\n<!-- END aayudha:skills -->\n\n" +
			"## Our own section\n\nMentions <!-- BEGIN aayudha:hosts --> inline, which is not a marker line.\n\n\n"
		got := string(RenderAgentsMD([]byte(existing), skills, hostsSec))
		want := "# Product agents\n\nOur rules.\n\n<!-- BEGIN aayudha:skills -->\n## Skills\n\nnew skills text\n<!-- END aayudha:skills -->\n\n" +
			"## Our own section\n\nMentions <!-- BEGIN aayudha:hosts --> inline, which is not a marker line.\n\n" +
			"<!-- BEGIN aayudha:hosts -->\n## Hosts\n\nhosts text\n<!-- END aayudha:hosts -->\n"
		if got != want {
			t.Fatalf("got:\n%q\nwant:\n%q", got, want)
		}
		if again := string(RenderAgentsMD([]byte(got), skills, hostsSec)); again != got {
			t.Fatalf("second render differs:\n%q", again)
		}
	})

	t.Run("remove a section, keep the rest", func(t *testing.T) {
		product := "# Product agents\n\nOur rules.\n"
		with := RenderAgentsMD([]byte(product), AgentsSection{ID: "work-items", Body: "## Work items"}, skills)
		got := string(RemoveAgentsSection(with, "work-items"))
		want := string(RenderAgentsMD([]byte(product), skills))
		if got != want {
			t.Fatalf("got:\n%q\nwant:\n%q", got, want)
		}
		if back := string(RemoveAgentsSection(RemoveAgentsSection(with, "work-items"), "skills")); back != product {
			t.Fatalf("removing every section: %q, want the product's own text %q", back, product)
		}
	})
}
