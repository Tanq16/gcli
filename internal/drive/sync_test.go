package drive

import (
	"errors"
	"slices"
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
	plan, err := BuildPlan(local, newTree(), false, noHash, nil)
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
	plan, err := BuildPlan(newTree(), remote, false, noHash, nil)
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
	plan, err := BuildPlan(newTree(), remote, false, noHash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(relList(plan.Deletes), []string{"a"}) {
		t.Fatalf("deletes = %v, want [a]", relList(plan.Deletes))
	}
}

func TestBuildPlanMkDirsOrdering(t *testing.T) {
	local := mkTree(nil, "a/b/c", "a", "a/b")
	plan, err := BuildPlan(local, newTree(), false, noHash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.MkDirs, []string{"a", "a/b", "a/b/c"}) {
		t.Fatalf("MkDirs = %v (want shallowest-first)", plan.MkDirs)
	}
}

func TestBuildPlanSkippedPassthrough(t *testing.T) {
	skipped := []string{"notes/Plan", "link"}
	plan, err := BuildPlan(newTree(), newTree(), false, noHash, skipped)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Skipped, skipped) {
		t.Fatalf("Skipped = %v", plan.Skipped)
	}
	// A skipped remote path is never keyed into the tree, so it can never appear in Deletes.
	if len(plan.Deletes) != 0 {
		t.Fatalf("skipped path leaked into deletes: %v", relList(plan.Deletes))
	}
}

func TestBuildPlanEmpty(t *testing.T) {
	plan, err := BuildPlan(newTree(), newTree(), false, noHash, nil)
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
	plan, err := BuildPlan(local, remoteSame, false, noHash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 0 {
		t.Fatalf("identical single file should be OpNone, got %v", relList(plan.Files))
	}

	remoteDiff := mkTree(map[string]Entry{"notes.md": {Size: 200, MTime: baseTime, MD5: "aa", ID: "r"}})
	plan, err = BuildPlan(local, remoteDiff, false, noHash, nil)
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
	plan, err := BuildPlan(local, remote, true, noHash, nil)
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
	plan2, err := BuildPlan(local, remote2, true, noHash, nil)
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
	plan, err := BuildPlan(local, remote, false, hash, nil)
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
	_, err := BuildPlan(local, remote, false, func(string) (string, error) { return "", errors.New("io") }, nil)
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
