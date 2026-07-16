package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/spf13/pflag"
)

func init() {
	os.Setenv("GH_PROMPT_DISABLED", "1")
	os.Setenv("NO_COLOR", "1")
	os.Setenv("CLICOLOR", "0")
}

func runGh(args ...string) (string, string, error) {
	stdout, stderr, err := gh.Exec(args...)
	return stdout.String(), stderr.String(), err
}

// ghRun runs a gh command and prints its output. It exits on error.
func ghRun(args ...string) {
	if err := runGhWithErr(args...); err != nil {
		os.Exit(1)
	}
}

// runGhWithErr runs a gh command, prints its output, and returns any error.
func runGhWithErr(args ...string) error {
	out, errOut, err := runGh(args...)
	if out != "" {
		fmt.Fprint(os.Stdout, out)
	}
	if errOut != "" {
		fmt.Fprint(os.Stderr, errOut)
	}
	return err
}

type PR struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	BaseRefName string `json:"baseRefName"`
	IsDraft     bool   `json:"isDraft"`
}

type Branch struct {
	Name        string `json:"name"`
	Head        string `json:"head"`
	Base        string `json:"base"`
	IsCurrent   bool   `json:"isCurrent"`
	IsMerged    bool   `json:"isMerged"`
	IsQueued    bool   `json:"isQueued"`
	NeedsRebase bool   `json:"needsRebase"`
}

type Stack struct {
	Trunk     string   `json:"trunk"`
	Current   string   `json:"currentBranch"`
	Branches  []Branch `json:"branches"`
}

func ghStackView() Stack {
	out, _, err := gh.Exec("extension", "exec", "stack", "view", "--json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load stack view:", err)
		os.Exit(1)
	}
	var stack Stack
	if err := json.Unmarshal(out.Bytes(), &stack); err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse stack view:", err)
		os.Exit(1)
	}
	return stack
}

func ghPrView(branch string) (*PR, error) {
	out, errOut, err := gh.Exec("pr", "view", branch, "--json", "number,state,baseRefName,isDraft")
	if err != nil {
		// only a missing PR is the nil case; everything else is a real error
		errText := strings.ToLower(errOut.String())
		if strings.Contains(errText, "no pull requests found") {
			return nil, nil
		}
		return nil, err
	}
	outStr := strings.TrimSpace(out.String())
	if outStr == "" {
		return nil, nil
	}
	var pr PR
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

func gitCommitsBetween(base, branch string) ([]string, error) {
	rev := base + ".." + branch
	out, err := exec.Command("git", "log", "--format=%s", "--reverse", "--end-of-options", rev).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", rev, err)
	}
	var commits []string
	for _, line := range strings.Split(string(out), "\n") {
		if c := strings.TrimSpace(line); c != "" {
			commits = append(commits, c)
		}
	}
	return commits, nil
}

func prTitleAndBody(branch, base string) (string, string, error) {
	commits, err := gitCommitsBetween(base, branch)
	if err != nil {
		return "", "", err
	}
	if len(commits) == 0 {
		return "", "", nil
	}
	if len(commits) == 1 {
		return commits[0], "", nil
	}
	return commits[0], strings.Join(commits[1:], "\n"), nil
}

func repoInfo() (current, parent string, err error) {
	stdout, _, err := gh.Exec("repo", "view", "--json", "nameWithOwner,parent")
	if err != nil {
		return "", "", fmt.Errorf("gh repo view: %w", err)
	}
	var result struct {
		NameWithOwner string `json:"nameWithOwner"`
		Parent        struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"parent"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", "", fmt.Errorf("parse repo view: %w", err)
	}
	return result.NameWithOwner, result.Parent.NameWithOwner, nil
}

func remoteInfo(remote string) (owner, repo string, err error) {
	if remote == "" {
		// prefer origin; otherwise pick the first git remote
		if out, err := exec.Command("git", "remote").Output(); err == nil {
			remotes := strings.Fields(string(out))
			for _, r := range remotes {
				if r == "origin" {
					remote = r
					break
				}
			}
			if remote == "" && len(remotes) > 0 {
				remote = remotes[0]
			}
		}
	}
	if remote == "" {
		return "", "", fmt.Errorf("no git remote configured")
	}
	url, err := exec.Command("git", "remote", "get-url", "--push", remote).Output()
	if err != nil {
		return "", "", fmt.Errorf("git remote get-url --push %s: %w", remote, err)
	}
	return parseGitURL(strings.TrimSpace(string(url)))
}

func parseGitURL(raw string) (owner, repo string, err error) {
	// https://github.com/owner/repo.git
	// ssh://git@github.com/owner/repo.git
	// git@github.com:owner/repo.git
	if strings.HasSuffix(raw, ".git") {
		raw = raw[:len(raw)-4]
	}
	if i := strings.Index(raw, "://"); i != -1 {
		raw = raw[i+3:]
	}
	if i := strings.Index(raw, "@"); i != -1 {
		raw = raw[i+1:]
	}
	// scp-style git@host:owner/repo -> host/owner/repo
	if i := strings.Index(raw, ":"); i != -1 {
		raw = raw[:i] + "/" + raw[i+1:]
	}
	parts := strings.Split(raw, "/")
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) >= 2 {
		return clean[len(clean)-2], clean[len(clean)-1], nil
	}
	return "", "", fmt.Errorf("could not parse remote url: %s", raw)
}

func isOrgOwner(owner string) (bool, error) {
	out, errOut, err := gh.Exec("api", "orgs/"+owner, "--jq", ".type")
	if err == nil {
		return strings.TrimSpace(out.String()) == "Organization", nil
	}
	if strings.Contains(strings.ToLower(errOut.String()), "not found") ||
		strings.Contains(strings.ToLower(errOut.String()), "404") {
		return false, nil
	}
	return false, fmt.Errorf("gh api orgs/%s: %w", owner, err)
}

func ensurePRBase(pr *PR, base string) error {
	if pr == nil || pr.State != "OPEN" {
		return nil
	}
	if pr.BaseRefName == base {
		return nil
	}
	return runGhWithErr("pr", "edit", strconv.Itoa(pr.Number), "--base", base)
}

func pushStack(remote string) {
	args := []string{"extension", "exec", "stack", "push"}
	if remote != "" {
		args = append(args, "--remote", remote)
	}
	ghRun(args...)
}

func cmdSubmit(args []string) {
	fs := pflag.NewFlagSet("submit", pflag.ExitOnError)
	auto := fs.Bool("auto", true, "Use auto-generated titles (default).")
	open := fs.Bool("open", false, "Mark PRs as ready for review.")
	draft := fs.Bool("draft", false, "Create PRs as drafts (default without --open).")
	remote := fs.String("remote", "", "Remote to push to.")
	fs.Parse(args)

	if *draft {
		*open = false
	}

	pushStack(*remote)
	stack := ghStackView()
	if len(stack.Branches) == 0 {
		fmt.Println("No branches in stack.")
		return
	}

	current, parent, err := repoInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine repo info: %v\n", err)
		os.Exit(1)
	}
	baseRepo := current
	if parent != "" {
		baseRepo = parent
	}
	baseOwner := baseRepo
	if i := strings.Index(baseRepo, "/"); i != -1 {
		baseOwner = baseRepo[:i]
	}
	headOwner, headRepoName, err := remoteInfo(*remote)
	if err != nil {
		// no remote configured or unparsable; let gh pr create infer from repo
		fmt.Fprintf(os.Stderr, "warning: could not resolve remote info: %v\n", err)
		headOwner = ""
		headRepoName = ""
	}
	headRepo := ""
	if headOwner != "" && headRepoName != "" {
		headRepo = headOwner + "/" + headRepoName
	}

	errors := 0
	for i, br := range stack.Branches {
		base := stack.Trunk
		if i > 0 {
			base = stack.Branches[i-1].Name
		}
		pr, err := ghPrView(br.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to look up PR for %s: %v\n", br.Name, err)
			errors++
			continue
		}

		switch {
		case pr != nil && pr.State == "OPEN":
			if err := ensurePRBase(pr, base); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to update PR for %s: %v\n", br.Name, err)
				errors++
				continue
			}
			if *open && pr.IsDraft {
				if err := runGhWithErr("pr", "ready", strconv.Itoa(pr.Number)); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to mark PR for %s as ready: %v\n", br.Name, err)
					errors++
					continue
				}
			}
			fmt.Printf("Updated PR #%d for %s -> %s\n", pr.Number, br.Name, base)
		case pr != nil:
			fmt.Printf("PR for %s is %s, skipping\n", br.Name, pr.State)
		default:
			head := br.Name
			crossRepo := headRepo != "" && headRepo != baseRepo
			useAPI := false
			if crossRepo {
				if headOwner == baseOwner {
					// same-owner fork: disambiguate the head repo via the API
					useAPI = true
				} else {
					org, err := isOrgOwner(headOwner)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to determine if %s is an organization: %v\n", headOwner, err)
						errors++
						continue
					}
					useAPI = org
				}
			}

			title := ""
			body := ""
			if *auto || useAPI {
				var err error
				title, body, err = prTitleAndBody(br.Name, base)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to read commits for %s: %v\n", br.Name, err)
					errors++
					continue
				}
				if title == "" {
					fmt.Printf("Skipping %s: no commits to create a PR\n", br.Name)
					continue
				}
			}

			if useAPI {
				apiArgs := []string{
					"api", "--method", "POST", "repos/" + baseRepo + "/pulls",
					"-f", "title=" + title,
					"-f", "body=" + body,
					"-f", "base=" + base,
					"-f", "head=" + headOwner + ":" + br.Name,
					"-f", "head_repo=" + headRepo,
				}
				if !*open {
					apiArgs = append(apiArgs, "-F", "draft=true")
				}
				apiArgs = append(apiArgs, "--jq", ".html_url")
				if err := runGhWithErr(apiArgs...); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create PR for %s: %v\n", br.Name, err)
					errors++
					continue
				}
			} else {
				if headOwner != "" && headOwner != baseOwner {
					head = headOwner + ":" + br.Name
				}
				createArgs := []string{"pr", "create", "--repo", baseRepo, "--base", base, "--head", head}
				if *auto {
					createArgs = append(createArgs, "--title", title, "--body", body)
				} else {
					createArgs = append(createArgs, "--fill")
				}
				if !*open {
					createArgs = append(createArgs, "--draft")
				}
				if err := runGhWithErr(createArgs...); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create PR for %s: %v\n", br.Name, err)
					errors++
					continue
				}
			}
			fmt.Printf("Created PR for %s -> %s\n", br.Name, base)
		}
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "submit finished with %d error(s)\n", errors)
		os.Exit(1)
	}
}

func cmdSync(args []string) {
	fs := pflag.NewFlagSet("sync", pflag.ExitOnError)
	remote := fs.String("remote", "", "Remote to sync with.")
	fs.Parse(args)

	syncArgs := []string{"extension", "exec", "stack", "sync"}
	if *remote != "" {
		syncArgs = append(syncArgs, "--remote", *remote)
	}
	ghRun(syncArgs...)

	stack := ghStackView()
	errors := 0
	for i, br := range stack.Branches {
		base := stack.Trunk
		if i > 0 {
			base = stack.Branches[i-1].Name
		}
		pr, err := ghPrView(br.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to look up PR for %s: %v\n", br.Name, err)
			errors++
			continue
		}
		if err := ensurePRBase(pr, base); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update PR for %s: %v\n", br.Name, err)
			errors++
			continue
		}
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "sync finished with %d error(s)\n", errors)
		os.Exit(1)
	}
}

func cmdMerge(args []string) {
	fs := pflag.NewFlagSet("merge", pflag.ExitOnError)
	squash := fs.Bool("squash", false, "Use squash merge.")
	rebase := fs.Bool("rebase", false, "Use rebase merge.")
	noDelete := fs.Bool("no-delete-branch", false, "Do not delete branches after merge.")
	admin := fs.Bool("admin", false, "Use admin privileges to merge.")
	auto := fs.Bool("auto", false, "Enable auto-merge instead of merging immediately.")
	fs.Parse(args)

	if *squash && *rebase {
		fmt.Fprintln(os.Stderr, "error: --squash and --rebase are mutually exclusive")
		os.Exit(1)
	}

	stack := ghStackView()
	if len(stack.Branches) == 0 {
		fmt.Println("No stack to merge.")
		return
	}

	var method []string
	if *squash {
		method = []string{"--squash"}
	} else if *rebase {
		method = []string{"--rebase"}
	} else {
		method = []string{"--merge"}
	}

	var deleteFlag, adminFlag, autoFlag []string
	if !*noDelete {
		deleteFlag = []string{"--delete-branch"}
	}
	if *admin {
		adminFlag = []string{"--admin"}
	}
	if *auto {
		autoFlag = []string{"--auto"}
	}

	errors := 0
	for i := len(stack.Branches) - 1; i >= 0; i-- {
		br := stack.Branches[i]
		pr, err := ghPrView(br.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to look up PR for %s: %v\n", br.Name, err)
			errors++
			continue
		}
		if pr == nil {
			fmt.Printf("No open PR for %s, skipping\n", br.Name)
			continue
		}
		if pr.State != "OPEN" {
			fmt.Printf("PR for %s is %s, skipping\n", br.Name, pr.State)
			continue
		}
		mergeArgs := []string{"pr", "merge", strconv.Itoa(pr.Number)}
		mergeArgs = append(mergeArgs, method...)
		mergeArgs = append(mergeArgs, deleteFlag...)
		mergeArgs = append(mergeArgs, adminFlag...)
		mergeArgs = append(mergeArgs, autoFlag...)
		if err := runGhWithErr(mergeArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to merge PR for %s: %v\n", br.Name, err)
			errors++
			continue
		}
		fmt.Printf("Merged PR #%d for %s\n", pr.Number, br.Name)
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "merge finished with %d error(s)\n", errors)
		os.Exit(1)
	}
}

func passthrough(args []string) {
	ghRun(append([]string{"extension", "exec", "stack"}, args...)...)
}

func main() {
	if len(os.Args) < 2 {
		ghRun("extension", "exec", "stack")
		return
	}

	switch os.Args[1] {
	case "submit":
		cmdSubmit(os.Args[2:])
	case "sync":
		cmdSync(os.Args[2:])
	case "merge":
		cmdMerge(os.Args[2:])
	case "view", "version", "--version", "-v":
		passthrough(os.Args[1:])
	default:
		passthrough(os.Args[1:])
	}
}
