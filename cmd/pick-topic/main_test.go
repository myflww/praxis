// Copyright 2026 myflww
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeFixture(t *testing.T, root, relative, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func textFixture() string { return strings.Repeat("An ordinary source line with useful context.\n", 4) }

func invoke(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	status := run(args, &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

func gitFixture(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	command := exec.Command("git", append([]string{"-c", "core.fsmonitor=false"}, args...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func TestOptionsOnEitherSideAndLastSeedWins(t *testing.T) {
	options, err := parseArgs([]string{"--seed=1", "some-repo", "--exclude-path", "a.go", "--exclude-path=src", "--seed", "-27"})
	if err != nil {
		t.Fatal(err)
	}
	if options.repo != "some-repo" || !options.hasSeed || options.seed != -27 || !reflect.DeepEqual(options.excludes, []string{"a.go", "src"}) {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestHelpAndArgumentErrors(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		status, output, stderr := invoke(flag)
		if status != 0 || !strings.Contains(output, "Usage:") || stderr != "" {
			t.Fatalf("help %q: status=%d output=%q stderr=%q", flag, status, output, stderr)
		}
	}
	for _, args := range [][]string{
		{"--"}, {"--unknown"}, {"--seed"}, {"--seed=nope"}, {"--seed=9223372036854775808"},
		{"--exclude-path"}, {"one", "two"}, {""},
	} {
		status, output, stderr := invoke(args...)
		if status != 1 || output != "" || !strings.HasPrefix(stderr, "error:") {
			t.Errorf("args %q: status=%d output=%q stderr=%q", args, status, output, stderr)
		}
	}
	status, output, stderr := invoke("--help", "--unknown")
	if status != 0 || !strings.Contains(output, "Usage:") || stderr != "" {
		t.Fatalf("help before unknown option: status=%d output=%q stderr=%q", status, output, stderr)
	}
	status, output, stderr = invoke("--unknown", "--help")
	if status != 1 || output != "" || stderr == "" {
		t.Fatalf("unknown option before help: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestSingleCandidateAndDefaultDirectory(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/one.go", textFixture())
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	status, output, stderr := invoke()
	if status != 0 || output != "src/one.go\n" || stderr != "" {
		t.Fatalf("single candidate: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestEmptyAndInvalidRootDoNotLeakCandidates(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "tiny.txt", "small")
	file := writeFixture(t, root, "not-a-directory", "small")
	for _, path := range []string{root, filepath.Join(root, "missing"), file} {
		status, output, stderr := invoke(path)
		if status != 1 || output != "" || stderr == "" || strings.Contains(stderr, "tiny.txt") {
			t.Fatalf("invalid/empty root: status=%d output=%q stderr=%q", status, output, stderr)
		}
	}
}

func TestGitTrackedUntrackedIgnoreAndNestedRoot(t *testing.T) {
	root := t.TempDir()
	gitFixture(t, root, "init", "--quiet")
	writeFixture(t, root, "outside.go", textFixture())
	writeFixture(t, root, "nested/tracked.go", textFixture())
	gitFixture(t, root, "add", "nested/tracked.go")
	writeFixture(t, root, "nested/untracked.go", textFixture())
	writeFixture(t, root, "nested/ignored.go", textFixture())
	writeFixture(t, root, ".gitignore", "nested/tracked.go\nnested/ignored.go\n")
	paths, err := candidatePaths(filepath.Join(root, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, path := range paths {
		got[path] = true
	}
	if len(got) != 2 || !got["tracked.go"] || !got["untracked.go"] {
		t.Fatalf("unexpected nested Git paths: %#v", paths)
	}
	status, output, stderr := invoke(filepath.Join(root, "nested"), "--exclude-path", "untracked.go")
	if status != 0 || output != "tracked.go\n" || stderr != "" {
		t.Fatalf("tracked ignored selection: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestFallbackWithoutGitIsSilentAndDoesNotApplyIgnore(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "ignored.go", textFixture())
	writeFixture(t, root, ".gitignore", "ignored.go\n")
	t.Setenv("PATH", "")
	status, output, stderr := invoke(root)
	if status != 0 || output != "ignored.go\n" || stderr != "" {
		t.Fatalf("missing Git fallback: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestSingleGitFileLifecycleAndMissingGitFallback(t *testing.T) {
	root := t.TempDir()
	gitFixture(t, root, "init", "--quiet")
	file := writeFixture(t, root, "only.py", textFixture())
	ignore := writeFixture(t, root, ".gitignore", "only.py\n")
	check := func(t *testing.T, wantStatus int) {
		t.Helper()
		status, output, stderr := invoke(root)
		if status != wantStatus || (status == 0 && (output != "only.py\n" || stderr != "")) || (status != 0 && (output != "" || stderr == "")) {
			t.Fatalf("Git lifecycle: status=%d output=%q stderr=%q", status, output, stderr)
		}
	}
	t.Run("untracked-ignored", func(t *testing.T) { check(t, 1) })
	t.Run("missing-git-applies-no-ignore", func(t *testing.T) {
		t.Setenv("PATH", "")
		check(t, 0)
	})
	gitFixture(t, root, "add", "--force", "only.py")
	t.Run("tracked-ignored", func(t *testing.T) { check(t, 0) })
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	t.Run("tracked-deleted", func(t *testing.T) { check(t, 1) })
	gitFixture(t, root, "rm", "--cached", "--quiet", "only.py")
	if err := os.Remove(ignore); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "only.py", textFixture())
	t.Run("untracked-unignored", func(t *testing.T) { check(t, 0) })
}

func TestExcludesNormalizeAndDoNotInterpretGlobs(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.go", textFixture())
	writeFixture(t, root, "dir/b.go", textFixture())
	for _, exclusions := range [][]string{
		{"--exclude-path", "dir"},
		{"--exclude-path=./dir/../dir"},
		{"--exclude-path", filepath.Join(root, "dir")},
		{"--exclude-path", "dir/b.go"},
	} {
		status, output, stderr := invoke(append([]string{root}, exclusions...)...)
		if status != 0 || output != "a.go\n" || stderr != "" {
			t.Fatalf("exclude %q: status=%d output=%q stderr=%q", exclusions, status, output, stderr)
		}
	}
	status, output, stderr := invoke(root, "--exclude-path", "*", "--exclude-path", ".", "--exclude-path", "", "--exclude-path=", "--exclude-path", "a.go")
	if status != 0 || output != "dir/b.go\n" || stderr != "" {
		t.Fatalf("literal glob/root exclusion: status=%d output=%q stderr=%q", status, output, stderr)
	}
	status, output, stderr = invoke(root, "--exclude-path", filepath.Dir(root))
	if status != 1 || output != "" || stderr == "" {
		t.Fatalf("outside exclusion: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestSeedReproducibleAndExclusionPreventsRepeat(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one.go", "two.go", "three.go"} {
		writeFixture(t, root, name, textFixture())
	}
	status, selected, stderr := invoke(root, "--seed=42")
	if status != 0 || stderr != "" {
		t.Fatalf("first choice: status=%d stderr=%q", status, stderr)
	}
	for i := 0; i < 5; i++ {
		status, output, stderr := invoke("--seed", "42", root)
		if status != 0 || output != selected || stderr != "" {
			t.Fatalf("seed changed: status=%d output=%q stderr=%q", status, output, stderr)
		}
	}
	status, output, stderr := invoke(root, "--seed=42", "--exclude-path", strings.TrimSpace(selected))
	if status != 0 || output == selected || stderr != "" {
		t.Fatalf("excluded repeat: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestPathFilters(t *testing.T) {
	for _, path := range []string{
		"node_modules/lib/main.js", "vendor/lib/main.go", "dist/app.js", "build/result.txt", "target/code.rs",
		".cache/data.txt", "__pycache__/source.py", "third_party/lib.js", "deps/library.py", "env/config.py",
		"package-lock.json", "Cargo.lock", "go.sum", "photo.PNG", "binary.exe", "app.min.js", "bundle.js.map", "types.generated.go",
		"app.gen.js", "candidate.pb.go", "Candidate.g.cs", "app.min.css", "candidate_generated.py", "candidate.min.txt", "candidate.pb.h", "bun.lockb",
	} {
		if allowedPath(path) {
			t.Errorf("sensitive/generated path allowed: %q", path)
		}
	}
	for _, path := range []string{
		"src/main.go", "README.md", "tests/example_test.py", "assets/icon.svg", "data/example.json", "Cargo.toml",
		".env", ".env.local", "keys/example.pem", "example.key", "text.wasm", "generated/source.js", "candidate_gen.go", "Pods/code.js",
		"skills/demo/SKILL.md", ".codex/source.js", "example.docx", "archive.xz", "example.obj", "example.dat", "example.p12", "candidate_generated.go", "candidate.pb.js",
	} {
		if !allowedPath(path) {
			t.Errorf("source path excluded: %q", path)
		}
	}
	for _, path := range []string{"../escape.go", "/absolute.go", "line\nbreak.go", "tab\tname.go", "invalid\xff.go"} {
		if _, ok := cleanCandidatePath(path); ok {
			t.Errorf("unsafe path allowed: %q", path)
		}
	}
	for _, path := range []string{"del\x7fname.go", "next\u0085line.go", "separator\u2028name.go", "你好.go", "a b.go"} {
		if _, ok := cleanCandidatePath(path); !ok {
			t.Errorf("observed supported filename rejected: %q", path)
		}
	}
}

func TestFileSizeAndContentFilters(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name string
		body string
		want bool
	}{
		{"minimum.txt", strings.Repeat("a", minimumFileSize), true},
		{"small.txt", strings.Repeat("a", minimumFileSize-1), false},
		{"maximum.txt", strings.Repeat("a", maximumFileSize), true},
		{"large.txt", strings.Repeat("a", maximumFileSize+1), false},
		{"nul.txt", strings.Repeat("a", 100) + "\x00", false},
		{"invalid.txt", strings.Repeat("a", 100) + "\xff", false},
		{"truncated-eof.txt", strings.Repeat("a", 100) + "\xe7\x95", false},
		{"sample-split.txt", strings.Repeat("a", sampleSize-1) + "界" + strings.Repeat("a", 100), false},
		{"sample-invalid.txt", strings.Repeat("a", sampleSize-1) + "\xff" + strings.Repeat("a", 100), false},
		{"unicode.txt", strings.Repeat("普通中文正文。", 20), true},
		{"few-controls.txt", strings.Repeat("x", 981) + strings.Repeat("\x01", 19), true},
		{"many-controls.txt", strings.Repeat("x", 980) + strings.Repeat("\x01", 20), false},
		{"whitespace.txt", strings.Repeat("\t\n\r\f", 256), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			writeFixture(t, root, test.name, test.body)
			if got := usefulTextFile(root, test.name); got != test.want {
				t.Fatalf("usefulTextFile=%v, want %v", got, test.want)
			}
		})
	}
}

func TestExternalAndDirectorySymlinksRejectedRootSymlinkAccepted(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, root, "real.go", textFixture())
	writeFixture(t, outside, "external.go", textFixture())
	if err := os.Symlink(filepath.Join(outside, "external.go"), filepath.Join(root, "linked.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	if usefulTextFile(root, "linked.go") || usefulTextFile(root, "linked-dir/external.go") {
		t.Fatal("internal symlink was read")
	}
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	status, output, stderr := invoke(link)
	if status != 0 || output != "real.go\n" || stderr != "" {
		t.Fatalf("root symlink: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestInternalFileSymlinksAndChainsAreEligible(t *testing.T) {
	root := t.TempDir()
	root, err := canonicalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "actual.png", textFixture())
	if err := os.Symlink("actual.png", filepath.Join(root, "alias.js")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink("alias.js", filepath.Join(root, "chain.js")); err != nil {
		t.Fatal(err)
	}
	if !usefulTextFile(root, "alias.js") || !usefulTextFile(root, "chain.js") {
		t.Fatal("internal regular-file symlink was rejected")
	}
	status, output, stderr := invoke(root, "--exclude-path", "alias.js")
	if status != 0 || output != "chain.js\n" || stderr != "" {
		t.Fatalf("internal file alias: status=%d output=%q stderr=%q", status, output, stderr)
	}
	status, output, stderr = invoke(root, "--exclude-path", "actual.png", "--exclude-path", "chain.js")
	if status != 0 || output != "alias.js\n" || stderr != "" {
		t.Fatalf("lexical alias exclusion: status=%d output=%q stderr=%q", status, output, stderr)
	}
}

func TestFIFOIsSkippedWithoutBlocking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipe fixture uses mkfifo")
	}
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo unavailable")
	}
	root := t.TempDir()
	pipe := filepath.Join(root, "pipe.txt")
	if output, err := exec.Command(mkfifo, pipe).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v: %s", err, output)
	}
	writeFixture(t, root, "regular.go", textFixture())
	done := make(chan bool, 1)
	go func() {
		status, output, stderr := invoke(root)
		done <- status == 0 && output == "regular.go\n" && stderr == ""
	}()
	select {
	case okay := <-done:
		if !okay {
			t.Fatal("FIFO affected selection")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("FIFO blocked selection")
	}
}

func TestReadOnlyGitTreeAndIndexRemainUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission fixture")
	}
	root := t.TempDir()
	gitFixture(t, root, "init", "--quiet")
	file := writeFixture(t, root, "source.go", textFixture())
	gitFixture(t, root, "add", "source.go")
	index := filepath.Join(root, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755); _ = os.Chmod(file, 0o644) })
	status, output, stderr := invoke(root)
	if status != 0 || output != "source.go\n" || stderr != "" {
		t.Fatalf("read-only selection: status=%d output=%q stderr=%q", status, output, stderr)
	}
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("Git index changed")
	}
	if _, err := os.Stat(index + ".lock"); !os.IsNotExist(err) {
		t.Fatal("Git index lock was created")
	}
}
