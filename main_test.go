package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
