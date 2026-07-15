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
	out, errOut, err := runGh(args...)
	if out != "" {
		fmt.Fprint(os.Stdout, out)
	}
	if errOut != "" {
		fmt.Fprint(os.Stderr, errOut)
	}
	if err != nil {
		os.Exit(1)
	}
}

// ghRunIgnore runs a gh command and prints its output, but does not exit on error.
func ghRunIgnore(args ...string) {
	out, errOut, _ := runGh(args...)
	if out != "" {
		fmt.Fprint(os.Stdout, out)
	}
	if errOut != "" {
		fmt.Fprint(os.Stderr, errOut)
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
	Trunk        string   `json:"trunk"`
	CurrentBranch string  `json:"currentBranch"`
	Branches     []Branch `json:"branches"`
}

func ensureGhStack() {
	if _, errOut, err := runGh("extension", "exec", "stack", "--help"); err != nil {
		if errOut != "" {
			fmt.Fprint(os.Stderr, errOut)
		}
		fmt.Fprintln(os.Stderr, "github/gh-stack is not installed.")
		fmt.Fprintln(os.Stderr, "Install it first: gh extension install github/gh-stack")
		os.Exit(1)
	}
}

func ghStackView() Stack {
	out, errOut, err := runGh("extension", "exec", "stack", "view", "--json")
	if err != nil {
		if errOut != "" {
			fmt.Fprint(os.Stderr, errOut)
		}
		if out != "" {
			fmt.Fprint(os.Stdout, out)
		}
		os.Exit(1)
	}
	var s Stack
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse gh stack view --json output:", err)
		os.Exit(1)
	}
	return s
}

func ghPrView(branch string) (*PR, error) {
	out, errOut, err := runGh("pr", "view", branch, "--json", "number,state,baseRefName,isDraft")
	if err != nil {
		if strings.Contains(errOut, "no pull requests found") {
			return nil, nil
		}
		if errOut != "" {
			return nil, fmt.Errorf("%w: %s", err, errOut)
		}
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var pr PR
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}
	if pr.State == "" && pr.Number == 0 {
		return nil, nil
	}
	return &pr, nil
}

func gitCommitsBetween(base, branch string) []string {
	rev := base + ".." + branch
	out, err := exec.Command("git", "log", "--format=%s", "--reverse", "--end-of-options", rev).Output()
	if err != nil {
		return nil
	}
	var commits []string
	for _, line := range strings.Split(string(out), "\n") {
		if c := strings.TrimSpace(line); c != "" {
			commits = append(commits, c)
		}
	}
	return commits
}

func prTitleAndBody(branch, base string) (string, string) {
	commits := gitCommitsBetween(base, branch)
	if len(commits) == 0 {
		return "", ""
	}
	if len(commits) == 1 {
		return commits[0], ""
	}
	return commits[0], strings.Join(commits[1:], "\n")
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
			if pr.BaseRefName != base {
				if err := runGhWithErr("pr", "edit", strconv.Itoa(pr.Number), "--base", base); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to update PR for %s: %v\n", br.Name, err)
					errors++
					continue
				}
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
			createArgs := []string{"pr", "create", "--base", base, "--head", br.Name}
			if *auto {
				title, body := prTitleAndBody(br.Name, base)
				if title == "" {
					fmt.Printf("Skipping %s: no commits to create a PR\n", br.Name)
					continue
				}
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
		if pr != nil && pr.State == "OPEN" && pr.BaseRefName != base {
			if err := runGhWithErr("pr", "edit", strconv.Itoa(pr.Number), "--base", base); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to update PR for %s: %v\n", br.Name, err)
				errors++
				continue
			}
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
	ensureGhStack()

	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		fmt.Println("gh stackx: a wrapper around github/gh-stack")
		fmt.Println()
		fmt.Println("Commands overridden by this extension:")
		fmt.Println("  submit       create PRs with gh pr create (works without Stack preview)")
		fmt.Println("  sync         run gh stack sync, then fix PR base branches")
		fmt.Println("  merge        merge the whole stack top-down")
		fmt.Println()
		fmt.Println("All other commands are passed to: gh extension exec stack")
		fmt.Println()
		ghRun("extension", "exec", "stack", "--help")
		return
	}

	command := os.Args[1]
	rest := os.Args[2:]

	switch command {
	case "submit":
		cmdSubmit(rest)
	case "sync":
		cmdSync(rest)
	case "merge":
		cmdMerge(rest)
	default:
		passthrough(append([]string{command}, rest...))
	}
}
