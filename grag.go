package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// 1. CLI & Configuration
// =============================================================================

type Config struct {
	Root           string
	Output         string
	AbsOutput      string
	Excludes       map[string]bool
	ExcludeContent map[string]bool
	IncludeHidden  bool
	Flat           bool
	Absolute       bool
	Quiet          bool
	NoTimestamp    bool
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func parseConfig() (Config, error) {
	var cfg Config
	var excludes stringSlice
	var excludeContents stringSlice

	flag.StringVar(&cfg.Output, "o", "./DIRECTORY.md", "Output Markdown file")
	flag.StringVar(&cfg.Output, "output", "./DIRECTORY.md", "Output Markdown file")
	flag.Var(&excludes, "e", "Subdirectory to prune completely (repeatable)")
	flag.Var(&excludes, "exclude", "Subdirectory to prune completely (repeatable)")
	flag.Var(&excludeContents, "c", "Directory to exclude contents from walking, keeping the directory itself (repeatable)")
	flag.Var(&excludeContents, "exclude-content", "Directory to exclude contents from walking, keeping the directory itself (repeatable)")
	flag.BoolVar(&cfg.IncludeHidden, "include-hidden", false, "Include dotfiles/dot-directories")
	flag.BoolVar(&cfg.Flat, "flat", false, "Emit a flat list instead of a nested tree")
	flag.BoolVar(&cfg.Absolute, "absolute", false, "Emit absolute paths instead of root-relative")
	flag.BoolVar(&cfg.Quiet, "q", false, "Suppress per-entry warnings")
	flag.BoolVar(&cfg.Quiet, "quiet", false, "Suppress per-entry warnings")
	flag.BoolVar(&cfg.NoTimestamp, "no-timestamp", false, "Omit timestamp from header for determinism")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: drag [flags] <root-dir>\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		return cfg, fmt.Errorf("exactly one <root-dir> is required")
	}

	cfg.Root = args[0]

	info, err := os.Stat(cfg.Root)
	if err != nil {
		return cfg, fmt.Errorf("cannot access root: %w", err)
	}
	if !info.IsDir() {
		return cfg, fmt.Errorf("<root-dir> must be a directory")
	}

	absOutput, err := filepath.Abs(cfg.Output)
	if err != nil {
		return cfg, fmt.Errorf("cannot resolve output path: %w", err)
	}
	cfg.AbsOutput = absOutput

	cfg.Excludes = make(map[string]bool)
	for _, ex := range excludes {
		norm := normalizeExclude(ex)
		if norm != "" {
			cfg.Excludes[norm] = true
		}
	}

	cfg.ExcludeContent = make(map[string]bool)
	for _, ec := range excludeContents {
		norm := normalizeExclude(ec)
		if norm != "" {
			cfg.ExcludeContent[norm] = true
		}
	}

	return cfg, nil
}

func normalizeExclude(ex string) string {
	ex = filepath.ToSlash(filepath.Clean(ex))
	ex = strings.TrimPrefix(ex, "./")
	ex = strings.TrimSuffix(ex, "/")
	return ex
}

// =============================================================================
// 2. Walk
// =============================================================================

type Entry struct {
	Rel   string
	IsDir bool
}

func runWalk(cfg Config) ([]Entry, error) {
	var entries []Entry
	matchedExcludes := make(map[string]bool)
	matchedExcludeContents := make(map[string]bool)

	err := filepath.WalkDir(cfg.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == cfg.Root {
				return fmt.Errorf("cannot read root directory: %w", err)
			}
			if !cfg.Quiet {
				fmt.Fprintf(os.Stderr, "drag: cannot read %s: %v\n", path, err)
			}
			rel, _ := filepath.Rel(cfg.Root, path)
			entries = append(entries, Entry{Rel: filepath.ToSlash(rel), IsDir: true})
			return filepath.SkipDir
		}

		if path == cfg.Root {
			return nil
		}

		rel, err := filepath.Rel(cfg.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		absPath, _ := filepath.Abs(path)
		if absPath == cfg.AbsOutput {
			return nil
		}

		if !cfg.IncludeHidden && isHidden(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if cfg.Excludes[rel] {
			matchedExcludes[rel] = true
			info, infoErr := d.Info()
			isDir := infoErr == nil && info.IsDir()
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if cfg.ExcludeContent[rel] {
			matchedExcludeContents[rel] = true
			info, infoErr := d.Info()
			isDir := infoErr == nil && info.IsDir()
			entries = append(entries, Entry{Rel: rel, IsDir: isDir})
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		info, infoErr := d.Info()
		isDir := infoErr == nil && info.IsDir()

		entries = append(entries, Entry{Rel: rel, IsDir: isDir})
		return nil
	})

	if err != nil {
		return nil, err
	}

	if !cfg.Quiet {
		for ex := range cfg.Excludes {
			if !matchedExcludes[ex] {
				fmt.Fprintf(os.Stderr, "drag: warning: exclude target '%s' not found\n", ex)
			}
		}
		for ec := range cfg.ExcludeContent {
			if !matchedExcludeContents[ec] {
				fmt.Fprintf(os.Stderr, "drag: warning: exclude-content target '%s' not found\n", ec)
			}
		}
	}

	return entries, nil
}

func isHidden(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// =============================================================================
// 3. Render
// =============================================================================

type TreeNode struct {
	Name     string
	IsDir    bool
	Children []*TreeNode
}

func buildTree(entries []Entry) *TreeNode {
	root := &TreeNode{Name: ".", IsDir: true}
	nodeMap := map[string]*TreeNode{"": root}

	for _, e := range entries {
		if e.IsDir {
			parts := strings.Split(e.Rel, "/")
			curr := ""
			parent := root
			for _, p := range parts {
				if curr == "" {
					curr = p
				} else {
					curr += "/" + p
				}
				if _, ok := nodeMap[curr]; !ok {
					nodeMap[curr] = &TreeNode{Name: p, IsDir: true}
					parent.Children = append(parent.Children, nodeMap[curr])
				}
				parent = nodeMap[curr]
			}
		}
	}

	for _, e := range entries {
		if !e.IsDir {
			parts := strings.Split(e.Rel, "/")
			fileName := parts[len(parts)-1]
			dirPath := ""
			if len(parts) > 1 {
				dirPath = strings.Join(parts[:len(parts)-1], "/")
			}
			parent := nodeMap[dirPath]
			if parent == nil {
				parent = root
			}
			parent.Children = append(parent.Children, &TreeNode{Name: fileName, IsDir: false})
		}
	}

	sortTree(root)
	return root
}

func buildTreeAbs(entries []Entry, root string) *TreeNode {
	rootAbs, _ := filepath.Abs(root)
	rootNode := &TreeNode{Name: filepath.Base(rootAbs), IsDir: true}
	nodeMap := map[string]*TreeNode{rootAbs: rootNode}

	for _, e := range entries {
		fullPath, _ := filepath.Abs(filepath.Join(root, e.Rel))
		dirPath := filepath.Dir(fullPath)
		name := filepath.Base(fullPath)

		parent, ok := nodeMap[dirPath]
		if !ok {
			parent = rootNode
		}

		node := &TreeNode{Name: name, IsDir: e.IsDir}
		parent.Children = append(parent.Children, node)
		nodeMap[fullPath] = node
	}

	sortTree(rootNode)
	return rootNode
}

func sortTree(n *TreeNode) {
	sort.Slice(n.Children, func(i, j int) bool {
		if n.Children[i].IsDir != n.Children[j].IsDir {
			return n.Children[i].IsDir
		}
		return strings.ToLower(n.Children[i].Name) < strings.ToLower(n.Children[j].Name)
	})
	for _, child := range n.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

// renderAsciiTree draws the nested directory structure in ASCII format
func renderAsciiTree(w io.Writer, node *TreeNode, prefix string, isRoot bool) {
	if isRoot {
		name := node.Name
		if node.IsDir && name != "." {
			name += "/"
		}
		fmt.Fprintf(w, "%s\n", name)
	}

	for i, child := range node.Children {
		isLast := (i == len(node.Children)-1)

		name := child.Name
		if child.IsDir {
			name += "/"
		}

		fmt.Fprintf(w, "%s|---- %s\n", prefix, name)

		if child.IsDir && len(child.Children) > 0 {
			nextPrefix := prefix + "|       "
			if isLast {
				nextPrefix = prefix + "        "
			}
			renderAsciiTree(w, child, nextPrefix, false)
		}
	}
}

func render(cfg Config, entries []Entry) error {
	dirCount := 0
	fileCount := 0
	for _, e := range entries {
		if e.IsDir {
			dirCount++
		} else {
			fileCount++
		}
	}

	outFile, err := os.Create(cfg.Output)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer outFile.Close()

	displayRoot := cfg.Root
	if cfg.Absolute {
		displayRoot, _ = filepath.Abs(cfg.Root)
	}
	fmt.Fprintf(outFile, "# Directory Structure: `%s`\n\n", displayRoot)

	if !cfg.NoTimestamp {
		fmt.Fprintf(outFile, "> **Generated:** %s | **Directories:** %d | **Files:** %d\n\n",
			time.Now().Format("2006-01-02 15:04:05"), dirCount, fileCount)
	} else {
		fmt.Fprintf(outFile, "> **Directories:** %d | **Files:** %d\n\n", dirCount, fileCount)
	}

	fmt.Fprintln(outFile, "```txt")
	if cfg.Flat {
		renderFlat(outFile, entries, cfg.Absolute, cfg.Root)
	} else {
		var tree *TreeNode
		if cfg.Absolute {
			tree = buildTreeAbs(entries, cfg.Root)
		} else {
			tree = buildTree(entries)
			rootName := filepath.Base(cfg.Root)
			if rootName == "." || rootName == "" {
				absRoot, _ := filepath.Abs(cfg.Root)
				rootName = filepath.Base(absRoot)
			}
			tree.Name = rootName
		}

		renderAsciiTree(outFile, tree, "", true)
	}
	fmt.Fprintln(outFile, "```")
	return nil
}

func renderFlat(w io.Writer, entries []Entry, absolute bool, root string) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Rel) < strings.ToLower(entries[j].Rel)
	})

	for _, e := range entries {
		name := e.Rel
		if absolute {
			name, _ = filepath.Abs(filepath.Join(root, e.Rel))
		}
		if e.IsDir {
			name += "/"
		}
		fmt.Fprintf(w, "- `%s`\n", escapeBackticks(name))
	}
}

func escapeBackticks(s string) string {
	return strings.ReplaceAll(s, "`", "\\`")
}

// =============================================================================
// 4. Main
// =============================================================================

func main() {
	cfg, err := parseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "drag: %v\n", err)
		fmt.Fprintln(os.Stderr, "Usage: drag [flags] <root-dir>")
		os.Exit(2)
	}

	entries, err := runWalk(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drag: %v\n", err)
		os.Exit(1)
	}

	err = render(cfg, entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drag: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
