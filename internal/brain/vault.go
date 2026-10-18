package brain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// HomePath is the vault's front door.
const HomePath = "Home.md"

// MarkerPath marks a branch as an aayudha brain (Obsidian hides dot files).
// .obsidian/types.json cannot: Obsidian writes it in any vault with properties.
const MarkerPath = ".aayudha-brain"

// Marker is the marker's exact content.
const Marker = "aayudha brain: aayu brain checkpoint refreshes the generated blocks of these notes.\n"

// TimelinePath is the dated record from inception to now.
const TimelinePath = "Timeline/Timeline.md"

// mocFolders lists the root folders that get a map of content: every type
// folder except Timeline, which is one note.
func mocFolders() []string {
	return slices.DeleteFunc(Folders(), func(f string) bool { return f == "Timeline" })
}

// mocIntro is the person-written line under each map's title.
var mocIntro = map[string]string{
	"Vision":       "The vision and the mission of the product.",
	"Principles":   "The founding principles. Each note gives the principle, the reason, and how we apply it.",
	"Guidance":     "Guidance for the people and twins that do the work.",
	"Direction":    "Where the product goes next, and what it will not do.",
	"Objectives":   "What we try to achieve, and how we measure it. Each note links its Goal and Initiative issues.",
	"Objections":   "Concerns and cases against a direction, principle, objective, or decision. We keep them after they are answered.",
	"Decisions":    "The durable reasons behind each decision. The Decision issue stays the record.",
	"Capabilities": "What the product does and why. Each note links its Feature issues.",
	"Personas":     "The people the product serves.",
	"Problems":     "The problems worth solving, with evidence.",
	"Glossary":     "The terms of the product, one note for each term.",
	"Seats":        "The fourteen seats of the pod. A seat owns notes, so knowledge stays when a person leaves.",
	"People":       "The people who work on the product, by GitHub login.",
	"Workstreams":  "Each workstream has its own index and map, linked up to this vault.",
	"Releases":     "One note for each release: the plan, the changes since the baseline, and what shipped.",
	"Revenue":      "Revenue and pricing knowledge.",
}

// revenuePublic is the Revenue map's rule on a public repository.
const revenuePublic = `This repository is public, so this vault is public. Do not write pricing, revenue, customer names, or unreleased strategy here.
Write only facts that are already public. Anything else needs the founder's agreement first.
`

func newNote(t Type, id, title, today string, sources []string, up string) *Note {
	n := &Note{}
	n.Set("id", id)
	n.Set("type", t.Name)
	n.Set("title", title)
	n.Set("owner", string(t.Owner))
	n.Set("status", t.Statuses[0])
	n.Set("updated", today)
	n.Set("sources", nonNil(sources))
	if up != "" {
		n.Set("up", up)
	}
	n.Set("tags", []string{"type/" + t.Name})
	return n
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func mocNote(folder, title, intro, up, today string) []byte {
	t, _ := FindType("moc")
	n := newNote(t, "moc-"+slug(title), title, today, nil, up)
	n.Body = "# " + title + "\n\n" + intro + "\n\n" + Block("notes", "") + "\n"
	return n.Render()
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(slugRE.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// Skeleton is the vault `aayu brain init` creates for repo: Home, a map of
// content per folder, the Timeline, the templates, and the Obsidian config.
// public says whether the repository is public (the Revenue rule).
func Skeleton(repo, today string, public bool) map[string][]byte {
	files := map[string][]byte{MarkerPath: []byte(Marker)}
	home, _ := FindType("home")
	h := newNote(home, "home", "Start here", today, nil, "")
	h.Body = homeBody(repo)
	files[HomePath] = h.Render()
	for _, f := range mocFolders() {
		intro := mocIntro[f]
		if f == "Revenue" && public {
			intro += "\n\n" + revenuePublic
		}
		files[MOCPath(f)] = mocNote(f, f, intro, "[[Home]]", today)
	}
	tl, _ := FindType("timeline")
	t := newNote(tl, "timeline", "Timeline", today, []string{"https://github.com/" + repo}, "[[Home]]")
	t.Body = "# Timeline\n\nThe product from inception to now. The dated lines come from the repository's records.\n\n## Eras\n\n" + Block("events", "") + "\n"
	files[TimelinePath] = t.Render()
	for _, ty := range Types {
		if ty.Folder == "" || ty.Name == "timeline" {
			continue
		}
		files[TemplatePath(ty)] = Template(ty)
	}
	for p, b := range obsidianConfig() {
		files[p] = b
	}
	files[".gitignore"] = []byte("# Per-person Obsidian state: never shared.\n.obsidian/workspace.json\n.obsidian/workspace-mobile.json\n.obsidian/workspaces.json\n.obsidian/hotkeys.json\n.obsidian/appearance.json\n.obsidian/community-plugins.json\n.obsidian/plugins/\n.obsidian/cache/\n.trash/\n")
	files[".gitattributes"] = []byte("* text=auto eol=lf\n")
	return files
}

// Workstream is the small brain of workstream ws: its Index (linked up to the
// root Workstreams map) and its own map of content.
func Workstream(repo, ws, today string) map[string][]byte {
	t, _ := FindType("workstream")
	idx := newNote(t, "workstream-"+ws, ws, today, []string{"https://github.com/" + repo}, "[[Workstreams]]")
	idx.Set("workstream", ws)
	idx.Body = "# " + ws + "\n\nThe start page of the " + ws + " workstream. It links up to [[Home]].\n\n" +
		"## Purpose\n\n## Scope\n\n## Start here\n\n1. Read [[" + ws + " MOC]] for every note of this workstream.\n"
	moc := mocNote("", ws+" MOC", "Every note of the "+ws+" workstream links up here.", "[["+ws+" Index]]", today)
	m, _ := Parse("", moc)
	m.Set("workstream", ws)
	return map[string][]byte{WorkstreamIndex(ws): idx.Render(), WorkstreamMOC(ws): m.Render()}
}

// WorkstreamName is a valid workstream folder name.
var WorkstreamName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

func homeBody(repo string) string {
	return "# Start here\n\n" +
		"This vault is the product brain of " + repo + ". It keeps the product knowledge that must outlive any one person.\n" +
		"Issues stay the record of the work. The notes here link to issues and do not copy them.\n\n" +
		"## The path for a new person\n\n" +
		"1. [[Vision]]: why the product exists, and its mission.\n" +
		"2. [[Principles]]: the rules we keep.\n" +
		"3. [[Personas]] and [[Problems]]: who we serve, and what gets in their way.\n" +
		"4. [[Direction]] and [[Objectives]]: where we go, and how we measure it.\n" +
		"5. [[Capabilities]]: what the product does, and why.\n" +
		"6. [[Decisions]] and [[Objections]]: what we chose, and the cases against it.\n" +
		"7. [[Releases]] and [[Timeline]]: how the product got here.\n" +
		"8. [[Glossary]], [[Guidance]], [[Seats]], and [[People]]: the words, the ways of work, and the people.\n\n" +
		"## How this vault works\n\n" +
		"Open the brain branch as a folder in Obsidian. The graph view colours each note by its type.\n" +
		"People and twins write the notes. `aayu brain checkpoint` refreshes only the marked generated blocks.\n" +
		"Every note names its sources. `aayu brain check` finds broken links, missing sources, and long sentences.\n\n" +
		"## Workstreams\n\n" + Block("workstreams", "") + "\n\n" +
		"## Releases\n\n" + Block("releases", "") + "\n\n" +
		"## Needs review\n\nNotes whose generated facts changed after a person last reviewed them.\n\n" + Block("review", "") + "\n"
}

// Template is the Obsidian template of note type t (core Templates plugin:
// {{title}} and {{date:FORMAT}} are filled in when a person inserts it).
func Template(t Type) []byte {
	n := &Note{}
	n.Set("id", t.Name+"-{{date:YYYYMMDDHHmm}}")
	n.Set("type", t.Name)
	n.Set("title", "{{title}}")
	n.Set("owner", string(t.Owner))
	n.Set("people", []string{})
	n.Set("status", t.Statuses[0])
	n.Set("updated", "{{date:YYYY-MM-DD}}")
	n.Set("reviewed", "{{date:YYYY-MM-DD}}")
	n.Set("sources", []string{})
	up := "[[" + t.Folder + "]]"
	if t.Name == "workstream" {
		n.Set("workstream", "")
	}
	n.Set("up", up)
	for _, k := range t.Keys {
		switch k {
		case "issues", "objectives", "objections", "supersedes", "superseded_by":
			n.Set(k, []string{})
		default:
			n.Set(k, "")
		}
	}
	n.Set("tags", []string{"type/" + t.Name})
	var b strings.Builder
	b.WriteString("# {{title}}\n\n")
	for _, s := range t.Sections {
		fmt.Fprintf(&b, "## %s\n\n", s)
	}
	n.Body = b.String()
	return n.Render()
}

// obsidianConfig is the shared, CLI-owned Obsidian configuration: link
// format, core plugins, templates folder, property types, and a graph that
// colours notes by type.
func obsidianConfig() map[string][]byte {
	js := func(v any) []byte { b, _ := json.MarshalIndent(v, "", "  "); return append(b, '\n') }
	type color struct {
		A   int `json:"a"`
		RGB int `json:"rgb"`
	}
	type group struct {
		Query string `json:"query"`
		Color color  `json:"color"`
	}
	var groups []group
	for _, t := range Types {
		groups = append(groups, group{Query: "tag:#type/" + t.Name, Color: color{A: 1, RGB: t.Color}})
	}
	types := map[string]string{
		"id": "text", "type": "text", "title": "text", "owner": "text", "people": "multitext", "status": "text",
		"workstream": "text", "updated": "date", "reviewed": "date", "sources": "multitext", "up": "text",
		"issues": "multitext", "objectives": "multitext", "objections": "multitext", "supersedes": "multitext",
		"superseded_by": "multitext", "raised_by": "text", "objects_to": "text", "resolution": "text",
		"direction": "text", "version": "text", "release_issue": "number", "aliases": "aliases", "tags": "tags",
	}
	return map[string][]byte{
		".obsidian/app.json": js(map[string]any{
			"newLinkFormat": "shortest", "alwaysUpdateLinks": true, "useMarkdownLinks": false,
			"userIgnoreFilters": []string{"Templates/"}, "showFrontmatter": true,
		}),
		".obsidian/core-plugins.json": js(map[string]bool{
			"file-explorer": true, "global-search": true, "graph": true, "backlink": true, "outgoing-link": true,
			"tag-pane": true, "properties": true, "page-preview": true, "templates": true, "outline": true,
		}),
		".obsidian/templates.json": js(map[string]string{"folder": "Templates", "dateFormat": "YYYY-MM-DD"}),
		".obsidian/types.json":     js(map[string]any{"types": types}),
		".obsidian/graph.json": js(map[string]any{
			"colorGroups": groups, "showTags": false, "showAttachments": false, "hideUnresolved": false,
			"showOrphans": true, "search": "-path:Templates",
		}),
	}
}
