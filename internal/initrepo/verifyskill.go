package initrepo

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The control tool templates under hosts/verify-skill/, by the stack a
// checkout already has (WP4.5b.3): a product's verify skill never brings a
// runtime the product does not use.
var controlTools = []struct {
	marker, stack, src, dst string
	argv                    func(dir string) []string
}{
	{"go.mod", "go", "go/control/main.go", "control/main.go", func(dir string) []string { return []string{"go", "run", "./" + dir + "/control"} }},
	{"package.json", "node", "node/control.mjs", "control.mjs", func(dir string) []string { return []string{"node", dir + "/control.mjs"} }},
	{"", "other", "other/control.sh", "control.sh", func(dir string) []string { return []string{"sh", dir + "/control.sh"} }},
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// VerifySkillDir is where init --verify-skill writes a repository's verify
// skill: .agents/skills/verify-<repository name>.
func VerifySkillDir(repoName string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(repoName), "-"), "-")
	if slug == "" {
		slug = "product"
	}
	return ".agents/skills/verify-" + slug
}

// VerifySkill renders the files of a product's verify skill, by repository
// path: SKILL.md, a feature map with no features yet, and a control tool in the
// checkout's own stack (go.mod: Go; else package.json: Node; else a stub to
// replace). It also returns the stack it picked.
func VerifySkill(assets fs.FS, root, repoName string) (map[string][]byte, string, error) {
	dir := VerifySkillDir(repoName)
	tool := controlTools[len(controlTools)-1]
	for _, t := range controlTools[:len(controlTools)-1] {
		if _, err := os.Stat(filepath.Join(root, t.marker)); err == nil {
			tool = t
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
	}
	argv := tool.argv(dir)
	argvJSON, err := json.Marshal(argv)
	if err != nil {
		return nil, "", err
	}
	product := strings.TrimPrefix(dir, ".agents/skills/verify-")
	fill := strings.NewReplacer("{{product}}", product, "{{control}}", strings.Join(argv, " "), "{{control_json}}", string(argvJSON))
	out := map[string][]byte{}
	for src, dst := range map[string]string{"SKILL.md": "SKILL.md", "features/README.md": "features/README.md", tool.src: tool.dst} {
		b, err := fs.ReadFile(assets, "hosts/verify-skill/"+src)
		if err != nil {
			return nil, "", err
		}
		if strings.HasSuffix(src, ".md") {
			b = []byte(fill.Replace(string(b)))
		}
		out[dir+"/"+dst] = b
	}
	return out, tool.stack, nil
}

func (in *Init) verifySkill(_ context.Context, x Extras) error {
	if x.Assets == nil {
		return errors.New("this build has no embedded templates")
	}
	files, stack, err := VerifySkill(x.Assets, in.Root, in.Repo.Name)
	if err != nil {
		return err
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if err := in.writeFile("verify-skill", p, files[p]); err != nil {
			return err
		}
	}
	in.note("verify-skill", "file", VerifySkillDir(in.Repo.Name), "unchanged",
		"control tool for a "+stack+" product; add the features with the verify skill's create procedure, then `aayu verify --map`")
	return nil
}
