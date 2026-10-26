// Copyright 2026 myflww
// SPDX-License-Identifier: Apache-2.0

// pick-topic chooses one useful text file inside a user-selected directory.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minimumFileSize = 80
	maximumFileSize = 1_000_000
	sampleSize      = 16 * 1024
)

const usage = `Usage: pick-topic [REPO] [--exclude-path PATH ...] [--seed INTEGER]

Choose one useful text file and print its repository-relative POSIX path.
REPO defaults to the current directory. Options may precede or follow REPO.

  --exclude-path PATH  Exclude a file or directory; may be repeated.
  --seed INTEGER       Reproduce a choice for the same input and implementation.
  -h, --help           Show this help.
`

type options struct {
	repo     string
	excludes []string
	seed     int64
	hasSeed  bool
	help     bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	if opts.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	root, err := canonicalRoot(opts.repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	requestedRoot := opts.repo
	if requestedRoot == "" {
		requestedRoot = "."
	}
	requestedRoot, _ = filepath.Abs(requestedRoot)
	excludes, err := normalizeExcludes(root, requestedRoot, opts.excludes)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	paths, err := candidatePaths(root)
	if err != nil {
		fmt.Fprintln(stderr, "error: unable to enumerate the selected directory")
		return 1
	}
	eligible := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		rel, ok := cleanCandidatePath(path)
		if !ok || seen[rel] || excludedPath(rel, excludes) || !allowedPath(rel) {
			continue
		}
		seen[rel] = true
		if usefulTextFile(root, rel) {
			eligible = append(eligible, rel)
		}
	}
	if len(eligible) == 0 {
		fmt.Fprintln(stderr, "error: no eligible text files found")
		return 1
	}
	// Sorting makes a seed independent of filesystem and Git enumeration order.
	sort.Strings(eligible)
	seed := opts.seed
	if !opts.hasSeed {
		seed = time.Now().UnixNano()
	}
	chosen := eligible[rand.New(rand.NewSource(seed)).Intn(len(eligible))]
	if _, err := fmt.Fprintln(stdout, chosen); err != nil {
		fmt.Fprintln(stderr, "error: unable to write the selected path")
		return 1
	}
	return 0
}

func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" {
			opts.help = true
			return opts, nil
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "--exclude-path" || name == "--seed" {
			if !hasValue {
				i++
				if i >= len(args) {
					return opts, fmt.Errorf("%s requires a value", name)
				}
				value = args[i]
			}
			if name == "--exclude-path" {
				if value != "" {
					opts.excludes = append(opts.excludes, value)
				}
			} else {
				seed, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return opts, errors.New("--seed requires an integer in the signed 64-bit range")
				}
				opts.seed, opts.hasSeed = seed, true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unknown option %q", arg)
		}
		if opts.repo != "" {
			return opts, errors.New("only one repository path may be supplied")
		}
		if arg == "" {
			return opts, errors.New("repository path must not be empty")
		}
		opts.repo = arg
	}
	return opts, nil
}

func canonicalRoot(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("unable to resolve the repository path")
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", errors.New("repository must be an existing directory")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("repository must be an existing directory")
	}
	return root, nil
}

func normalizeExcludes(root, requestedRoot string, raw []string) ([]string, error) {
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		path := filepath.FromSlash(item)
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		} else if aliasRelative, err := filepath.Rel(requestedRoot, path); err == nil && aliasRelative != ".." && !strings.HasPrefix(aliasRelative, ".."+string(filepath.Separator)) {
			// A symlink used for REPO is a supported spelling of the same root.
			path = filepath.Join(root, aliasRelative)
		}
		// Exclusions refer to the displayed lexical path: excluding a symlink
		// alias must not accidentally exclude its differently named target.
		rel, err := filepath.Rel(root, filepath.Clean(path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, errors.New("excluded paths must be inside the selected directory")
		}
		if rel != "." {
			result = append(result, filepath.ToSlash(rel))
		}
	}
	return result, nil
}

func excludedPath(path string, excludes []string) bool {
	for _, excluded := range excludes {
		candidate, prefix := path, excluded
		if runtime.GOOS == "windows" {
			candidate, prefix = strings.ToLower(candidate), strings.ToLower(prefix)
		}
		if candidate == prefix || strings.HasPrefix(candidate, prefix+"/") {
			return true
		}
	}
	return false
}

func candidatePaths(root string) ([]string, error) {
	cmd := exec.Command("git", "-c", "core.fsmonitor=false", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	cmd.Dir = root
	output, err := cmd.Output()
	if err == nil {
		parts := bytes.Split(output, []byte{0})
		paths := make([]string, 0, len(parts))
		for _, part := range parts {
			if len(part) > 0 {
				paths = append(paths, string(part))
			}
		}
		return paths, nil
	}
	// A plain folder and a machine without Git both remain usable. The fallback
	// deliberately does not attempt to interpret Git ignore syntax.
	return safeWalk(root)
}

func safeWalk(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !allowedPath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() || entry.Type()&os.ModeSymlink != 0 {
			paths = append(paths, rel)
		}
		return nil
	})
	return paths, err
}

func cleanCandidatePath(raw string) (string, bool) {
	if raw == "" || !utf8.ValidString(raw) || !filepath.IsLocal(raw) {
		return "", false
	}
	for _, char := range raw {
		if char < 32 {
			return "", false
		}
	}
	clean := filepath.Clean(filepath.FromSlash(raw))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(clean), true
}

var skippedDirectories = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "bower_components": true, "vendor": true, "vendors": true,
	"third_party": true, "third-party": true, "deps": true,
	"dist": true, "build": true, "out": true, "target": true, "coverage": true,
	".next": true, ".nuxt": true, ".svelte-kit": true,
	".cache": true, "__pycache__": true, ".venv": true, "venv": true, "env": true,
}

var skippedNames = map[string]bool{
	"package-lock.json": true, "npm-shrinkwrap.json": true, "pnpm-lock.yaml": true, "bun.lockb": true,
	"go.sum": true,
}

var skippedExtensions = map[string]bool{
	".7z": true, ".zip": true, ".gz": true, ".tar": true, ".rar": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".avif": true, ".ico": true, ".bmp": true, ".tif": true, ".tiff": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".mp4": true, ".mov": true, ".webm": true, ".avi": true,
	".pdf": true, ".xls": true, ".xlsx": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".o": true, ".class": true, ".jar": true,
	".pyc": true, ".bin": true, ".db": true, ".sqlite": true, ".sqlite3": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	".lock": true, ".map": true,
}

func allowedPath(path string) bool {
	parts := strings.Split(path, "/")
	for _, component := range parts[:len(parts)-1] {
		if skippedDirectories[strings.ToLower(component)] {
			return false
		}
	}
	name := strings.ToLower(parts[len(parts)-1])
	// Checking the last component as a directory name also lets safeWalk prune it.
	if skippedDirectories[name] || skippedNames[name] || skippedExtensions[strings.ToLower(filepath.Ext(name))] {
		return false
	}
	if strings.Contains(name, ".min.") || strings.Contains(name, ".generated.") ||
		strings.Contains(name, ".gen.") || strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".pb.h") ||
		strings.HasSuffix(name, ".g.cs") || strings.HasSuffix(name, "_generated.py") {
		return false
	}
	return true
}

func usefulTextFile(root, rel string) bool {
	// Directory symlinks are not traversed. File symlinks are eligible only when
	// their complete chain resolves to a regular file within the selected root.
	path := root
	var info fs.FileInfo
	parts := strings.Split(rel, "/")
	for index, part := range parts {
		path = filepath.Join(path, part)
		var err error
		info, err = os.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if index != len(parts)-1 {
				return false
			}
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return false
			}
			targetRelative, err := filepath.Rel(root, path)
			if err != nil || targetRelative == ".." || strings.HasPrefix(targetRelative, ".."+string(filepath.Separator)) {
				return false
			}
			info, err = os.Lstat(path)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return false
			}
		}
	}
	if info == nil || !info.Mode().IsRegular() || info.Size() < minimumFileSize || info.Size() > maximumFileSize {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() < minimumFileSize || opened.Size() > maximumFileSize {
		return false
	}
	sample, err := io.ReadAll(io.LimitReader(file, sampleSize))
	if err != nil || bytes.IndexByte(sample, 0) >= 0 {
		return false
	}
	controls := 0
	for _, value := range sample {
		if value < 32 && value != 8 && value != 9 && value != 10 && value != 12 && value != 13 {
			controls++
		}
	}
	if controls*50 >= len(sample) {
		return false
	}
	return utf8.Valid(sample)
}
