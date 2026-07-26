package drive

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var baseTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestFileAction(t *testing.T) {
	tests := []struct {
		name       string
		local      Entry
		remote     Entry
		hashVal    string
		hashErr    error
		wantOp     Op
		wantErr    bool
		wantCalled bool
	}{
		{"identical", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime, MD5: "aa"}, "", nil, OpNone, false, false},
		{"within window", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(500 * time.Millisecond), MD5: "aa"}, "", nil, OpNone, false, false},
		{"boundary equal window", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(time.Second), MD5: "aa"}, "", nil, OpNone, false, false},
		{"drift md5 match", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(2 * time.Second), MD5: "aa"}, "aa", nil, OpTouch, false, true},
		{"drift md5 differ", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(2 * time.Second), MD5: "bb"}, "aa", nil, OpUpdate, false, true},
		{"negative drift md5 differ", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(-2 * time.Second), MD5: "bb"}, "aa", nil, OpUpdate, false, true},
		{"size differs no hash", Entry{Size: 100, MTime: baseTime}, Entry{Size: 200, MTime: baseTime, MD5: "aa"}, "aa", nil, OpUpdate, false, false},
		{"hash error", Entry{Size: 100, MTime: baseTime}, Entry{Size: 100, MTime: baseTime.Add(2 * time.Second), MD5: "aa"}, "", errors.New("boom"), OpNone, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			hash := func() (string, error) {
				called = true
				return tt.hashVal, tt.hashErr
			}
			op, err := FileAction(tt.local, tt.remote, hash)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if op != tt.wantOp {
				t.Fatalf("op = %v, want %v", op, tt.wantOp)
			}
			if called != tt.wantCalled {
				t.Fatalf("hashLocal called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

func mkTree(files map[string]Entry, dirs ...string) *Tree {
	tr := newTree()
	for rel, e := range files {
		e.RelPath = rel
		tr.Files[rel] = e
	}
	for _, d := range dirs {
		tr.Dirs[d] = Entry{RelPath: d}
	}
	return tr
}

func noHash(string) (string, error) { return "", errors.New("hash should not be called") }

func opsByRel(items []Item) map[string]Op {
	m := map[string]Op{}
	for _, it := range items {
		m[it.RelPath] = it.Op
	}
	return m
}

func relList(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.RelPath
	}
	return out
}

func TestBuildPlanCreateOnly(t *testing.T) {
	local := mkTree(map[string]Entry{
		"a.txt":     {Size: 1, MTime: baseTime},
		"sub/b.txt": {Size: 2, MTime: baseTime},
	}, "sub")
	plan, err := BuildPlan(local, newTree(), PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.MkDirs, []string{"sub"}) {
		t.Fatalf("MkDirs = %v", plan.MkDirs)
	}
	ops := opsByRel(plan.Files)
	if ops["a.txt"] != OpCreate || ops["sub/b.txt"] != OpCreate {
		t.Fatalf("ops = %v", ops)
	}
	if len(plan.Deletes) != 0 {
		t.Fatalf("deletes = %v", relList(plan.Deletes))
	}
}

func TestBuildPlanDeleteOnly(t *testing.T) {
	remote := mkTree(map[string]Entry{
		"x.txt":     {Size: 1, MTime: baseTime, ID: "rx"},
		"old/y.txt": {Size: 2, MTime: baseTime, ID: "ry"},
	}, "old")
	plan, err := BuildPlan(newTree(), remote, PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	got := relList(plan.Deletes)
	slices.Sort(got)
	// old/y.txt is covered by the topmost "old" delete and must not appear.
	if !slices.Equal(got, []string{"old", "x.txt"}) {
		t.Fatalf("deletes = %v", got)
	}
	if len(plan.Files) != 0 || len(plan.MkDirs) != 0 {
		t.Fatalf("unexpected files/mkdirs")
	}
}

func TestBuildPlanTopmostDeleteMinimization(t *testing.T) {
	remote := mkTree(map[string]Entry{
		"a/d.txt":   {Size: 1, MTime: baseTime, ID: "1"},
		"a/b/c.txt": {Size: 2, MTime: baseTime, ID: "2"},
	}, "a", "a/b")
	plan, err := BuildPlan(newTree(), remote, PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(relList(plan.Deletes), []string{"a"}) {
		t.Fatalf("deletes = %v, want [a]", relList(plan.Deletes))
	}
}

func TestBuildPlanMkDirsOrdering(t *testing.T) {
	local := mkTree(nil, "a/b/c", "a", "a/b")
	plan, err := BuildPlan(local, newTree(), PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.MkDirs, []string{"a", "a/b", "a/b/c"}) {
		t.Fatalf("MkDirs = %v (want shallowest-first)", plan.MkDirs)
	}
}

// Skipping an entry drops it from its own tree but must also protect the same rel path
// on the other side, where it would otherwise look dest-only and be mirror-deleted.
func TestBuildPlanProtectSet(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]Entry
		dirs        []string
		protected   []string
		wantDeletes []string
	}{
		{
			"skipped local symlink shadows a remote file of the same name",
			map[string]Entry{"link": {ID: "1"}, "keep.txt": {ID: "2"}},
			nil,
			[]string{"link"},
			[]string{"keep.txt"},
		},
		{
			"dest-only folder holding only skipped items is never collapsed",
			map[string]Entry{"Other/x.txt": {ID: "1"}},
			[]string{"Reports", "Other"},
			[]string{"Reports/Q1"},
			[]string{"Other"},
		},
		{
			"protected folder still deletes its unprotected children",
			map[string]Entry{"Reports/notes.txt": {ID: "1"}},
			[]string{"Reports"},
			[]string{"Reports/Q1"},
			[]string{"Reports/notes.txt"},
		},
		{
			"protection walks up every ancestor directory",
			map[string]Entry{"a/b/other.txt": {ID: "1"}},
			[]string{"a", "a/b", "a/b/c"},
			[]string{"a/b/c/Doc"},
			[]string{"a/b/other.txt"},
		},
		{
			"unprotected subtrees still collapse to their topmost dir",
			map[string]Entry{"a/b/x.txt": {ID: "1"}},
			[]string{"a", "a/b", "z", "z/w"},
			[]string{"a/Doc"},
			[]string{"a/b", "z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protected := map[string]bool{}
			for _, p := range tt.protected {
				protected[p] = true
			}
			plan, err := BuildPlan(newTree(), mkTree(tt.files, tt.dirs...), PlanOptions{HashLocal: noHash, Protected: protected})
			if err != nil {
				t.Fatal(err)
			}
			if got := relList(plan.Deletes); !slices.Equal(got, tt.wantDeletes) {
				t.Fatalf("deletes = %v, want %v", got, tt.wantDeletes)
			}
		})
	}
}

func TestBuildPlanDeleteBlastRadius(t *testing.T) {
	remote := mkTree(map[string]Entry{
		"Archive/a.txt":      {ID: "1"},
		"Archive/deep/b.txt": {ID: "2"},
		"Archive/deep/c.txt": {ID: "3"},
		"loose.txt":          {ID: "4"},
	}, "Archive", "Archive/deep", "Empty")
	plan, err := BuildPlan(newTree(), remote, PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if got := relList(plan.Deletes); !slices.Equal(got, []string{"Archive", "Empty", "loose.txt"}) {
		t.Fatalf("deletes = %v", got)
	}
	want := map[string]Item{
		"Archive":   {IsDir: true, Descendants: 3},
		"Empty":     {IsDir: true, Descendants: 0},
		"loose.txt": {IsDir: false, Descendants: 0},
	}
	for _, it := range plan.Deletes {
		if w := want[it.RelPath]; it.IsDir != w.IsDir || it.Descendants != w.Descendants {
			t.Fatalf("%q: IsDir=%v Descendants=%d, want IsDir=%v Descendants=%d",
				it.RelPath, it.IsDir, it.Descendants, w.IsDir, w.Descendants)
		}
	}
	// Three collapsed files plus the loose one — not the three plan items the gate lists.
	if got := DeleteFileCount(plan.Deletes); got != 4 {
		t.Fatalf("DeleteFileCount = %d, want 4", got)
	}
}

// On a case-insensitive filesystem a case-only difference is one file, so a pull must
// not plan a download and a delete for it.
func TestBuildPlanCaseFoldReverse(t *testing.T) {
	local := mkTree(map[string]Entry{"readme.md": {Size: 5, MTime: baseTime}}, "docs")
	remote := mkTree(map[string]Entry{"README.md": {Size: 9, MTime: baseTime, MD5: "aa", ID: "r"}}, "Docs")
	tests := []struct {
		name        string
		same        func(a, b string) bool
		wantFiles   map[string]Op
		wantMkDirs  []string
		wantDeletes []string
	}{
		{
			"case-insensitive filesystem folds the pair into one update",
			func(a, b string) bool { return strings.EqualFold(a, b) },
			map[string]Op{"readme.md": OpUpdate},
			nil,
			nil,
		},
		{
			"case-sensitive filesystem keeps them distinct",
			nil,
			map[string]Op{"README.md": OpCreate},
			[]string{"Docs"},
			[]string{"docs", "readme.md"},
		},
		{
			"folded names on distinct inodes are not the same entry",
			func(string, string) bool { return false },
			map[string]Op{"README.md": OpCreate},
			[]string{"Docs"},
			[]string{"docs", "readme.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildPlan(local, remote, PlanOptions{Reverse: true, HashLocal: noHash, SameLocal: tt.same})
			if err != nil {
				t.Fatal(err)
			}
			if got := opsByRel(plan.Files); !maps.Equal(got, tt.wantFiles) {
				t.Fatalf("files = %v, want %v", got, tt.wantFiles)
			}
			if got := plan.MkDirs; !slices.Equal(got, tt.wantMkDirs) {
				t.Fatalf("mkdirs = %v, want %v", got, tt.wantMkDirs)
			}
			if got := relList(plan.Deletes); !slices.Equal(got, tt.wantDeletes) {
				t.Fatalf("deletes = %v, want %v", got, tt.wantDeletes)
			}
		})
	}
}

func TestBuildPlanEmpty(t *testing.T) {
	plan, err := BuildPlan(newTree(), newTree(), PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.MkDirs) != 0 || len(plan.Files) != 0 || len(plan.Deletes) != 0 {
		t.Fatalf("expected empty plan, got %+v", plan)
	}
}

func TestBuildPlanSingleFile(t *testing.T) {
	local := mkTree(map[string]Entry{"notes.md": {Size: 100, MTime: baseTime}})
	remoteSame := mkTree(map[string]Entry{"notes.md": {Size: 100, MTime: baseTime, MD5: "aa", ID: "r"}})
	plan, err := BuildPlan(local, remoteSame, PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 0 {
		t.Fatalf("identical single file should be OpNone, got %v", relList(plan.Files))
	}
	if plan.Unchanged != 1 {
		t.Fatalf("Unchanged = %d, want 1", plan.Unchanged)
	}

	remoteDiff := mkTree(map[string]Entry{"notes.md": {Size: 200, MTime: baseTime, MD5: "aa", ID: "r"}})
	plan, err = BuildPlan(local, remoteDiff, PlanOptions{HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 1 || plan.Files[0].Op != OpUpdate {
		t.Fatalf("size diff should be one OpUpdate, got %+v", plan.Files)
	}
}

func TestBuildPlanReverseDirection(t *testing.T) {
	// Reverse: remote is the source. A size-diff file must download (OpUpdate) with
	// Src carrying the remote entry (its ID), Dst the local entry.
	local := mkTree(map[string]Entry{"f": {Size: 100, MTime: baseTime}})
	remote := mkTree(map[string]Entry{"f": {Size: 200, MTime: baseTime, MD5: "aa", ID: "R1"}})
	plan, err := BuildPlan(local, remote, PlanOptions{Reverse: true, HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("want 1 file item, got %v", plan.Files)
	}
	it := plan.Files[0]
	if it.Op != OpUpdate || it.Src.ID != "R1" || it.Dst.Size != 100 {
		t.Fatalf("reverse item = %+v (want OpUpdate, Src=remote, Dst=local)", it)
	}

	// A remote-only file under reverse is a create (download).
	remote2 := mkTree(map[string]Entry{"f": {Size: 200, MTime: baseTime, MD5: "aa", ID: "R1"}, "new": {Size: 5, MTime: baseTime, ID: "R2"}})
	plan2, err := BuildPlan(local, remote2, PlanOptions{Reverse: true, HashLocal: noHash})
	if err != nil {
		t.Fatal(err)
	}
	if opsByRel(plan2.Files)["new"] != OpCreate {
		t.Fatalf("remote-only under reverse should create, got %v", plan2.Files)
	}
}

func TestBuildPlanTouch(t *testing.T) {
	local := mkTree(map[string]Entry{"f": {Size: 100, MTime: baseTime}})
	remote := mkTree(map[string]Entry{"f": {Size: 100, MTime: baseTime.Add(2 * time.Second), MD5: "matched", ID: "r"}})
	hash := func(string) (string, error) { return "matched", nil }
	plan, err := BuildPlan(local, remote, PlanOptions{HashLocal: hash})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 1 || plan.Files[0].Op != OpTouch {
		t.Fatalf("mtime drift + md5 match should touch, got %+v", plan.Files)
	}
}

func TestBuildPlanHashErrorPropagates(t *testing.T) {
	local := mkTree(map[string]Entry{"f": {Size: 100, MTime: baseTime}})
	remote := mkTree(map[string]Entry{"f": {Size: 100, MTime: baseTime.Add(2 * time.Second), MD5: "x", ID: "r"}})
	_, err := BuildPlan(local, remote, PlanOptions{HashLocal: func(string) (string, error) { return "", errors.New("io") }})
	if err == nil {
		t.Fatal("expected hash error to propagate")
	}
}

func TestCollisions(t *testing.T) {
	tests := []struct {
		name     string
		items    []namedID
		caseFold bool
		wantLen  int
	}{
		{"none", []namedID{{"a", "1"}, {"b", "2"}}, false, 0},
		{"exact dup", []namedID{{"a", "1"}, {"a", "2"}}, false, 1},
		{"case collision folded", []namedID{{"Readme.md", "1"}, {"readme.md", "2"}}, true, 1},
		{"case collision not folded", []namedID{{"Readme.md", "1"}, {"readme.md", "2"}}, false, 0},
		{"case fold ignores singletons", []namedID{{"a", "1"}, {"B", "2"}}, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collisions(tt.items, tt.caseFold)
			if len(got) != tt.wantLen {
				t.Fatalf("collisions = %v (len %d), want len %d", got, len(got), tt.wantLen)
			}
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		patterns []string
		want     bool
	}{
		{"log by base", "dir/app.log", []string{"*.log"}, true},
		{"log at root", "app.log", []string{"*.log"}, true},
		{"non-match", "app.txt", []string{"*.log"}, false},
		{"dir name base match", "a/node_modules", []string{"node_modules"}, true},
		{"path glob", "dir/x", []string{"dir/*"}, true},
		{"path glob other dir", "other/x", []string{"dir/*"}, false},
		{"empty patterns", "anything", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIgnore(tt.rel, tt.patterns); got != tt.want {
				t.Fatalf("shouldIgnore(%q, %v) = %v, want %v", tt.rel, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestParseIgnore(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"comma split", []string{"*.tmp,node_modules", "*.log"}, []string{"*.tmp", "node_modules", "*.log"}},
		{"empty", nil, nil},
		{"blanks dropped", []string{"  ", "", " a "}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseIgnore(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("parseIgnore(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestTrashDest(t *testing.T) {
	existing := map[string]bool{}
	exists := func(p string) bool { return existing[p] }

	got := trashDest("/t", "a/report.pdf", exists)
	if got != "/t/a/report.pdf" {
		t.Fatalf("no-collision dest = %q", got)
	}

	existing["/t/a/report.pdf"] = true
	got = trashDest("/t", "a/report.pdf", exists)
	if got != "/t/a/report (1).pdf" {
		t.Fatalf("first collision dest = %q, want '/t/a/report (1).pdf'", got)
	}

	existing["/t/a/report (1).pdf"] = true
	got = trashDest("/t", "a/report.pdf", exists)
	if got != "/t/a/report (2).pdf" {
		t.Fatalf("second collision dest = %q", got)
	}

	existing2 := map[string]bool{"/t/noext": true}
	got = trashDest("/t", "noext", func(p string) bool { return existing2[p] })
	if got != "/t/noext (1)" {
		t.Fatalf("extensionless collision dest = %q, want '/t/noext (1)'", got)
	}
}

func TestBuildLocalTreeReportsIgnored(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("keep.txt", "a")
	mustWrite("cache/only.tmp", "b")
	mustWrite("build/out.bin", "c")

	tree, _, ignored, err := buildLocalTree(t.Context(), root, parseIgnore([]string{"*.tmp", "build"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree.Files["keep.txt"]; !ok {
		t.Fatalf("keep.txt missing from tree: %v", tree.Files)
	}
	if _, ok := tree.Files["cache/only.tmp"]; ok {
		t.Fatal("ignored file leaked into the tree")
	}
	// Without these the other side sees them as dest-only; an ignored file, and a whole
	// pruned directory, would both be mirror-deleted.
	if !slices.Contains(ignored, "cache/only.tmp") {
		t.Fatalf("ignored file not reported: %v", ignored)
	}
	if !slices.Contains(ignored, "build") {
		t.Fatalf("pruned directory not reported: %v", ignored)
	}
}

// The probe answers for the volume, not the GOOS, and must survive a root that the pull
// has not created yet by walking up to the deepest existing ancestor.
func TestCaseInsensitiveDir(t *testing.T) {
	root := t.TempDir()
	got := caseInsensitiveDir(root)

	probe := filepath.Join(root, "Probe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(filepath.Join(root, "probe"))
	want := err == nil
	if got != want {
		t.Fatalf("caseInsensitiveDir = %v, but the volume itself reports %v", got, want)
	}
	if entries, derr := os.ReadDir(root); derr != nil {
		t.Fatal(derr)
	} else if len(entries) != 1 {
		t.Fatalf("probe left files behind: %d entries", len(entries))
	}

	if got != caseInsensitiveDir(filepath.Join(root, "not", "created", "yet")) {
		t.Fatal("a missing root must answer for its deepest existing ancestor")
	}
}

func TestHasAncestorIn(t *testing.T) {
	set := map[string]bool{"a": true, "a/b": true}
	tests := []struct {
		path string
		want bool
	}{
		{"a", false}, // itself is not a strict ancestor
		{"a/b", true},
		{"a/b/c", true},
		{"x/y", false},
		{"a2/b", false}, // prefix string but not path ancestor
	}
	for _, tt := range tests {
		if got := hasAncestorIn(tt.path, set); got != tt.want {
			t.Fatalf("hasAncestorIn(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
