package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestIsValidGitHubOwner(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple owner", "ThePlenkov", true},
		{"owner with hyphen", "cli-github", true},
		{"owner with underscore", "cli_github", false},
		{"empty string", "", false},
		{"too long", string(make([]byte, 40)), false},
		{"starts with hyphen", "-org", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidGitHubOwner(tt.input); got != tt.want {
				t.Errorf("isValidGitHubOwner(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidGitHubRepo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple repo", "gh-stackx", true},
		{"repo with dot", "gh.stackx", true},
		{"repo with hyphen", "gh-stackx", true},
		{"repo with underscore", "gh_stackx", true},
		{"empty string", "", false},
		{"too long", string(make([]byte, 101)), false},
		{"single dot", ".", false},
		{"double dot", "..", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidGitHubRepo(tt.input); got != tt.want {
				t.Errorf("isValidGitHubRepo(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitScpHostPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPath string
		wantOK   bool
	}{
		{"simple scp", "github.com:ThePlenkov/gh-stackx", "github.com", "ThePlenkov/gh-stackx", true},
		{"scp with .git", "github.com:ThePlenkov/gh-stackx.git", "github.com", "ThePlenkov/gh-stackx.git", true},
		{"ipv6", "[::1]:/path/to/repo", "[::1]", "/path/to/repo", true},
		{"no colon", "github.com/ThePlenkov/gh-stackx", "github.com/ThePlenkov/gh-stackx", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPath, gotOK := splitScpHostPath(tt.input)
			if gotHost != tt.wantHost || gotPath != tt.wantPath || gotOK != tt.wantOK {
				t.Errorf("splitScpHostPath(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, gotHost, gotPath, gotOK, tt.wantHost, tt.wantPath, tt.wantOK)
			}
		})
	}
}

func TestParseGitRemote(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https", "https://github.com/ThePlenkov/gh-stackx.git", "github.com", "ThePlenkov", "gh-stackx", false},
		{"ssh git", "git@github.com:ThePlenkov/gh-stackx.git", "github.com", "ThePlenkov", "gh-stackx", false},
		{"scp no user", "github.com:ThePlenkov/gh-stackx", "github.com", "ThePlenkov", "gh-stackx", false},
		{"gh enterprise https", "https://ghe.example.com/ThePlenkov/gh-stackx.git", "ghe.example.com", "ThePlenkov", "gh-stackx", false},
		{"no owner", "https://github.com/gh-stackx.git", "", "", "", true},
		{"invalid owner", "https://github.com/_ThePlenkov/gh-stackx.git", "", "", "", true},
		{"local path", "/home/ubuntu/repos/gh-stackx", "", "", "", true},
		{"windows drive", "C:/Users/dev/gh-stackx", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, err := parseGitRemote(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseGitRemote(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if host != tt.wantHost || owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("parseGitRemote(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.input, host, owner, repo, tt.wantHost, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestRepoWithHost(t *testing.T) {
	tests := []struct {
		host string
		repo string
		want string
	}{
		{"", "ThePlenkov/gh-stackx", "ThePlenkov/gh-stackx"},
		{"github.com", "ThePlenkov/gh-stackx", "github.com/ThePlenkov/gh-stackx"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := repoWithHost(tt.host, tt.repo); got != tt.want {
				t.Errorf("repoWithHost(%q, %q) = %q, want %q", tt.host, tt.repo, got, tt.want)
			}
		})
	}
}

func TestBaseForBranch(t *testing.T) {
	stack := Stack{
		Trunk:   "main",
		Current: "feature/api",
		Branches: []Branch{
			{Name: "feature/auth"},
			{Name: "feature/api"},
		},
	}
	tests := []struct {
		index int
		want  string
	}{
		{0, "main"},
		{1, "feature/auth"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("index-%d", tt.index), func(t *testing.T) {
			if got := baseForBranch(stack, tt.index); got != tt.want {
				t.Errorf("baseForBranch(stack, %d) = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}

func TestPrTitleAndBody(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")
	run("git", "commit", "--allow-empty", "-m", "first")
	run("git", "checkout", "-b", "feature")
	run("git", "commit", "--allow-empty", "-m", "feat: add auth")
	run("git", "commit", "--allow-empty", "-m", "fix: handle edge case")
	run("git", "commit", "--allow-empty", "-m", "docs: update README")

	title, commits, err := prTitleAndBody("feature", "main")
	if err != nil {
		t.Fatalf("prTitleAndBody: %v", err)
	}
	if title != "feat: add auth" {
		t.Errorf("title = %q, want %q", title, "feat: add auth")
	}
	wantCommits := []string{"fix: handle edge case", "docs: update README"}
	if len(commits) != len(wantCommits) {
		t.Fatalf("commits = %v, want %v", commits, wantCommits)
	}
	for i := range wantCommits {
		if commits[i] != wantCommits[i] {
			t.Errorf("commits[%d] = %q, want %q", i, commits[i], wantCommits[i])
		}
	}
}

func TestPrURL(t *testing.T) {
	tests := []struct {
		host   string
		repo   string
		number int
		want   string
	}{
		{"", "ThePlenkov/gh-stackx", 24, "https://github.com/ThePlenkov/gh-stackx/pull/24"},
		{"github.com", "ThePlenkov/gh-stackx", 24, "https://github.com/ThePlenkov/gh-stackx/pull/24"},
		{"ghe.example.com", "org/repo", 1, "https://ghe.example.com/org/repo/pull/1"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := prURL(tt.host, tt.repo, tt.number); got != tt.want {
				t.Errorf("prURL(%q, %q, %d) = %q, want %q", tt.host, tt.repo, tt.number, got, tt.want)
			}
		})
	}
}

func TestFormatStackTable(t *testing.T) {
	stack := Stack{
		Trunk:   "main",
		Current: "auth-layer",
		Branches: []Branch{
			{Name: "auth-layer"},
			{Name: "api-endpoints"},
			{Name: "frontend"},
		},
	}
	prs := map[string]branchPRInfo{
		"frontend":      {pr: &PR{Number: 32}, repo: "ThePlenkov/gh-stackx", host: "github.com"},
		"api-endpoints": {pr: &PR{Number: 30}, repo: "ThePlenkov/gh-stackx", host: "github.com"},
	}
	titles := map[string]string{
		"auth-layer":    "Add authentication layer",
		"api-endpoints": "Add API endpoints",
		"frontend":      "Add frontend",
	}

	got := formatStackTable(stack, "auth-layer", prs, titles)

	want := []string{
		"## Stack",
		"| # | Title | Pull request | Branch |",
		"|---|-------|--------------|--------|",
		"| 1 | Add frontend | [#32](https://github.com/ThePlenkov/gh-stackx/pull/32) | `frontend` |",
		"| 2 | Add API endpoints | [#30](https://github.com/ThePlenkov/gh-stackx/pull/30) | `api-endpoints` |",
		"| 3 | **Add authentication layer** |  | **`auth-layer`** |",
		"| | _main_ | | `main` |",
	}
	for _, line := range want {
		if !strings.Contains(got, line) {
			t.Errorf("formatStackTable() missing line %q in:\n%s", line, got)
		}
	}
}

func TestBuildPRBody(t *testing.T) {
	stack := Stack{
		Trunk:   "main",
		Current: "auth-layer",
		Branches: []Branch{
			{Name: "auth-layer"},
			{Name: "api-endpoints"},
		},
	}
	prs := map[string]branchPRInfo{
		"api-endpoints": {pr: &PR{Number: 30}, repo: "ThePlenkov/gh-stackx", host: "github.com"},
	}
	titles := map[string]string{
		"auth-layer":    "Add authentication layer",
		"api-endpoints": "Add API endpoints",
	}
	commits := []string{"feat: token helper", "fix: tests"}

	got := buildPRBody(stack, "auth-layer", prs, titles, commits)
	if !strings.Contains(got, "### Commits") {
		t.Errorf("buildPRBody() missing Commits section in:\n%s", got)
	}
	for _, want := range commits {
		if !strings.Contains(got, want) {
			t.Errorf("buildPRBody() missing commit subject %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "[#30](https://github.com/ThePlenkov/gh-stackx/pull/30)") {
		t.Errorf("buildPRBody() missing PR link in:\n%s", got)
	}
}

func TestManifestVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantRepo    string
		wantVersion string
	}{
		{
			name:        "full manifest",
			input:       "owner: ThePlenkov\nname: gh-stackx\nhost: github.com\ntag: v1.1.0\nispinned: false\npath: gh-stackx\n",
			wantRepo:    "ThePlenkov/gh-stackx",
			wantVersion: "v1.1.0",
		},
		{
			name:        "missing tag",
			input:       "owner: ThePlenkov\nname: gh-stackx\n",
			wantRepo:    "ThePlenkov/gh-stackx",
			wantVersion: "",
		},
		{
			name:        "missing owner/name falls back",
			input:       "tag: v1.0.0\n",
			wantRepo:    "ThePlenkov/gh-stackx",
			wantVersion: "v1.0.0",
		},
		{
			name:        "malformed lines ignored",
			input:       "owner: ThePlenkov\nnot a key value pair\nname: gh-stackx\ntag: v1.2.3\n",
			wantRepo:    "ThePlenkov/gh-stackx",
			wantVersion: "v1.2.3",
		},
		{
			name:        "empty manifest",
			input:       "",
			wantRepo:    "ThePlenkov/gh-stackx",
			wantVersion: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotVersion := manifestVersion([]byte(tt.input))
			if gotRepo != tt.wantRepo || gotVersion != tt.wantVersion {
				t.Errorf("manifestVersion(%q) = (%q, %q), want (%q, %q)",
					tt.input, gotRepo, gotVersion, tt.wantRepo, tt.wantVersion)
			}
		})
	}
}

func TestDevinPluginManifest(t *testing.T) {
	data, err := os.ReadFile(".devin-plugin/plugin.json")
	if err != nil {
		t.Fatalf("reading plugin.json: %v", err)
	}
	var plugin struct {
		Name    string   `json:"name"`
		Version string   `json:"version"`
		Skills  []string `json:"skills"`
	}
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("parsing plugin.json: %v", err)
	}
	if plugin.Name != "gh-stackx" {
		t.Errorf("name = %q, want %q", plugin.Name, "gh-stackx")
	}
	if plugin.Version == "" {
		t.Errorf("version is empty")
	}

	manifest, err := os.ReadFile("manifest.yml")
	if err != nil {
		t.Fatalf("reading manifest.yml: %v", err)
	}
	_, manifestVersion := manifestVersion(manifest)
	if strings.TrimPrefix(plugin.Version, "v") != strings.TrimPrefix(manifestVersion, "v") {
		t.Errorf("plugin version %q does not match manifest.yml tag %q", plugin.Version, manifestVersion)
	}

	if len(plugin.Skills) == 0 {
		t.Fatalf("skills field is empty")
	}
	for _, p := range plugin.Skills {
		skillFile := filepath.Join(p, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Errorf("skill file %q not found: %v", skillFile, err)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

func TestParseBoolPtr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *bool
		wantErr bool
	}{
		{"true", "true", boolPtr(true), false},
		{"yes", "yes", boolPtr(true), false},
		{"1", "1", boolPtr(true), false},
		{"false", "false", boolPtr(false), false},
		{"no", "no", boolPtr(false), false},
		{"0", "0", boolPtr(false), false},
		{"unset", "unset", nil, false},
		{"empty", "", nil, false},
		{"invalid", "maybe", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBoolPtr(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBoolPtr(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseBoolPtr(%q) = %v, want nil", tt.input, *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Errorf("parseBoolPtr(%q) = %v, want %v", tt.input, got, *tt.want)
			}
		})
	}
}

func TestSetAndGetConfigValue(t *testing.T) {
	cfg := &Config{}
	if err := setConfigValue(cfg, "submit.open", "true"); err != nil {
		t.Fatalf("setConfigValue submit.open: %v", err)
	}
	if err := setConfigValue(cfg, "merge.squash", "yes"); err != nil {
		t.Fatalf("setConfigValue merge.squash: %v", err)
	}
	if err := setConfigValue(cfg, "submit.labels", "bug,feature"); err != nil {
		t.Fatalf("setConfigValue submit.labels: %v", err)
	}

	if cfg.Submit.Open == nil || !*cfg.Submit.Open {
		t.Errorf("Submit.Open not set to true")
	}
	if cfg.Merge.Squash == nil || !*cfg.Merge.Squash {
		t.Errorf("Merge.Squash not set to true")
	}
	if !slices.Equal(cfg.Submit.Labels, []string{"bug", "feature"}) {
		t.Errorf("Submit.Labels = %v, want [bug feature]", cfg.Submit.Labels)
	}

	got, ok := getConfigValue(cfg, "submit.open")
	if !ok || got != "true" {
		t.Errorf("getConfigValue(submit.open) = (%q, %v), want (true, true)", got, ok)
	}
	got, ok = getConfigValue(cfg, "submit.labels")
	if !ok || got != "bug,feature" {
		t.Errorf("getConfigValue(submit.labels) = (%q, %v), want (bug,feature, true)", got, ok)
	}
	got, ok = getConfigValue(cfg, "submit.missing")
	if ok {
		t.Errorf("getConfigValue(submit.missing) unexpectedly found")
	}

	if err := setConfigValue(cfg, "submit.open", "unset"); err != nil {
		t.Fatalf("setConfigValue submit.open unset: %v", err)
	}
	if cfg.Submit.Open != nil {
		t.Errorf("Submit.Open = %v, want nil", *cfg.Submit.Open)
	}
}

func TestLoadConfigYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	content := `submit:
  draft: true
  remote: upstream
merge:
  squash: true
`
	if err := os.WriteFile(".gh-stackx.yaml", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if path != ".gh-stackx.yaml" {
		t.Errorf("loadConfig() path = %q, want .gh-stackx.yaml", path)
	}
	if cfg.Submit.Draft == nil || !*cfg.Submit.Draft {
		t.Errorf("Submit.Draft = %v, want true", cfg.Submit.Draft)
	}
	if cfg.Submit.Remote != "upstream" {
		t.Errorf("Submit.Remote = %q, want upstream", cfg.Submit.Remote)
	}
	if cfg.Merge.Squash == nil || !*cfg.Merge.Squash {
		t.Errorf("Merge.Squash = %v, want true", cfg.Merge.Squash)
	}
}

func TestLoadConfigJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	content := `{"sync":{"remote":"origin"}}`
	if err := os.WriteFile(".gh-stackx.json", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if path != ".gh-stackx.json" {
		t.Errorf("loadConfig() path = %q, want .gh-stackx.json", path)
	}
	if cfg.Sync.Remote != "origin" {
		t.Errorf("Sync.Remote = %q, want origin", cfg.Sync.Remote)
	}
}

func TestWriteAndLoadConfigYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Chdir(tmpDir)

	cfg := &Config{
		Submit: SubmitConfig{
			Open:   boolPtr(true),
			Remote: "upstream",
			Labels: []string{"enhancement"},
		},
		Merge: MergeConfig{
			Squash: boolPtr(false),
		},
	}
	if err := writeConfig(".gh-stackx.yaml", cfg); err != nil {
		t.Fatalf("writeConfig error = %v", err)
	}

	loaded, path, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if path != ".gh-stackx.yaml" {
		t.Errorf("loadConfig() path = %q, want .gh-stackx.yaml", path)
	}
	if loaded.Submit.Open == nil || !*loaded.Submit.Open {
		t.Errorf("loaded Submit.Open = %v, want true", loaded.Submit.Open)
	}
	if loaded.Submit.Remote != "upstream" {
		t.Errorf("loaded Submit.Remote = %q, want upstream", loaded.Submit.Remote)
	}
	if !slices.Equal(loaded.Submit.Labels, []string{"enhancement"}) {
		t.Errorf("loaded Submit.Labels = %v, want [enhancement]", loaded.Submit.Labels)
	}
	if loaded.Merge.Squash == nil || *loaded.Merge.Squash {
		t.Errorf("loaded Merge.Squash = %v, want false", loaded.Merge.Squash)
	}
}

func TestApplySubmitConfig(t *testing.T) {
	cfg := &Config{
		Submit: SubmitConfig{
			Open:   boolPtr(true),
			Auto:   boolPtr(false),
			Remote: "upstream",
		},
	}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	open := fs.Bool("open", false, "")
	draft := fs.Bool("draft", false, "")
	auto := fs.Bool("auto", true, "")
	remote := fs.String("remote", "", "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatal(err)
	}

	applySubmitConfig(cfg, fs, open, draft, auto, remote)
	if !*open {
		t.Errorf("open = %v, want true", *open)
	}
	if *draft {
		t.Errorf("draft = %v, want false", *draft)
	}
	if *auto {
		t.Errorf("auto = %v, want false", *auto)
	}
	if *remote != "upstream" {
		t.Errorf("remote = %q, want upstream", *remote)
	}

	// Flag overrides config.
	fs2 := pflag.NewFlagSet("test", pflag.ContinueOnError)
	open2 := fs2.Bool("open", false, "")
	draft2 := fs2.Bool("draft", false, "")
	auto2 := fs2.Bool("auto", true, "")
	remote2 := fs2.String("remote", "", "")
	if err := fs2.Parse([]string{"--draft"}); err != nil {
		t.Fatal(err)
	}
	applySubmitConfig(cfg, fs2, open2, draft2, auto2, remote2)
	if !*draft2 {
		t.Errorf("draft = %v, want true when --draft is passed", *draft2)
	}
	if *open2 {
		t.Errorf("open = %v, want false when --draft is passed", *open2)
	}
}

func TestApplyMergeConfig(t *testing.T) {
	cfg := &Config{
		Merge: MergeConfig{
			Squash:   boolPtr(true),
			NoDelete: boolPtr(true),
		},
	}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	squash := fs.Bool("squash", false, "")
	rebase := fs.Bool("rebase", false, "")
	noDelete := fs.Bool("no-delete-branch", false, "")
	admin := fs.Bool("admin", false, "")
	auto := fs.Bool("auto", false, "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatal(err)
	}

	applyMergeConfig(cfg, fs, squash, rebase, noDelete, admin, auto)
	if !*squash {
		t.Errorf("squash = %v, want true", *squash)
	}
	if *rebase {
		t.Errorf("rebase = %v, want false", *rebase)
	}
	if !*noDelete {
		t.Errorf("noDelete = %v, want true", *noDelete)
	}

	// Explicit --rebase overrides configured squash.
	fs2 := pflag.NewFlagSet("test", pflag.ContinueOnError)
	squash2 := fs2.Bool("squash", false, "")
	rebase2 := fs2.Bool("rebase", false, "")
	noDelete2 := fs2.Bool("no-delete-branch", false, "")
	admin2 := fs2.Bool("admin", false, "")
	auto2 := fs2.Bool("auto", false, "")
	if err := fs2.Parse([]string{"--rebase"}); err != nil {
		t.Fatal(err)
	}
	applyMergeConfig(cfg, fs2, squash2, rebase2, noDelete2, admin2, auto2)
	if *squash2 {
		t.Errorf("squash = %v, want false when --rebase is passed", *squash2)
	}
	if !*rebase2 {
		t.Errorf("rebase = %v, want true when --rebase is passed", *rebase2)
	}
}

func TestCmdConfigInit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cmdConfigInit([]string{})

	data, err := os.ReadFile(".gh-stackx.yaml")
	if err != nil {
		t.Fatalf("reading created config: %v", err)
	}
	content := string(data)
	for _, want := range []string{"submit:", "merge:", "# remote:"} {
		if !strings.Contains(content, want) {
			t.Errorf("created config missing %q\n%s", want, content)
		}
	}
}
