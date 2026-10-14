package cmd

import (
	"errors"
	"strings"
	"testing"
)

// fakeGit records git calls and returns canned outputs keyed by the first argument.
type fakeGit struct {
	calls   []string
	outputs map[string]string
	fails   map[string]bool
}

func (f *fakeGit) run(args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	if f.fails[args[0]] {
		return "", errors.New(args[0] + " failed")
	}
	return f.outputs[args[0]], nil
}

func TestPrepareBaseBranchUsesDefaultBranch(t *testing.T) {
	git := &fakeGit{outputs: map[string]string{"symbolic-ref": "origin/main\n"}}

	base, err := prepareBaseBranch(git.run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want main", base)
	}
	want := []string{
		"status --porcelain",
		"symbolic-ref --short refs/remotes/origin/HEAD",
		"checkout main",
		"pull --ff-only origin main",
	}
	if strings.Join(git.calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls = %v, want %v", git.calls, want)
	}
}

func TestPrepareBaseBranchFallsBackToMain(t *testing.T) {
	git := &fakeGit{fails: map[string]bool{"symbolic-ref": true}}

	base, err := prepareBaseBranch(git.run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want main", base)
	}
}

func TestPrepareBaseBranchStopsOnUncommittedChanges(t *testing.T) {
	git := &fakeGit{outputs: map[string]string{"status": " M public/a.md\n"}}

	if _, err := prepareBaseBranch(git.run); err == nil {
		t.Fatal("expected an error for uncommitted changes")
	}
	if len(git.calls) != 1 {
		t.Errorf("should stop right after git status, but called: %v", git.calls)
	}
}
