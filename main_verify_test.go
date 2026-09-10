package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateAndPasswordAPI(t *testing.T) {
	dataDir := t.TempDir()
	mux, err := buildMux(dataDir)
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}

	// 1. No state saved yet -> default empty state.
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"performers":[]`) {
		t.Fatalf("default GET /api/state = %d %q", rec.Code, rec.Body.String())
	}

	// 2. No password file -> blank submission verifies OK, wrong one does not.
	body, _ := json.Marshal(map[string]string{"password": ""})
	req = httptest.NewRequest(http.MethodPost, "/api/verify-password", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var vr struct{ Ok bool }
	json.Unmarshal(rec.Body.Bytes(), &vr)
	if !vr.Ok {
		t.Fatalf("expected ok=true for blank password with no password file, got false")
	}

	body, _ = json.Marshal(map[string]string{"password": "anything"})
	req = httptest.NewRequest(http.MethodPost, "/api/verify-password", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &vr)
	if vr.Ok {
		t.Fatalf("expected ok=false for non-blank password with no password file, got true")
	}

	// 3. POST a state blob, then GET it back.
	payload := []byte(`{"performers":[{"id":"p1","code":"A1","name":"Test"}],"activeId":"p1","fileName":"x"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/state", bytes.NewReader(payload))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/state = %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !bytes.Equal(bytes.TrimSpace(rec.Body.Bytes()), payload) {
		t.Fatalf("GET /api/state after save = %q, want %q", rec.Body.String(), payload)
	}

	// Confirm it actually persisted to disk (the whole point).
	onDisk, err := os.ReadFile(filepath.Join(dataDir, stateFileName))
	if err != nil || !bytes.Equal(bytes.TrimSpace(onDisk), payload) {
		t.Fatalf("state file on disk = %q, %v", onDisk, err)
	}

	// 4. Invalid JSON is rejected.
	req = httptest.NewRequest(http.MethodPost, "/api/state", strings.NewReader("not json"))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/state with bad JSON = %d, want 400", rec.Code)
	}

	// 5. Now add a password file and confirm the gate actually works.
	if err := os.WriteFile(filepath.Join(dataDir, passwordFileName), []byte("hunter2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	body, _ = json.Marshal(map[string]string{"password": "wrong"})
	req = httptest.NewRequest(http.MethodPost, "/api/verify-password", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &vr)
	if vr.Ok {
		t.Fatalf("wrong password should not verify")
	}

	body, _ = json.Marshal(map[string]string{"password": "hunter2"})
	req = httptest.NewRequest(http.MethodPost, "/api/verify-password", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &vr)
	if !vr.Ok {
		t.Fatalf("correct password (with trimmed trailing newline in file) should verify")
	}
}

func TestEnsurePasswordFile(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, passwordFileName)

	// 1. No file yet -> creates an empty one.
	if err := ensurePasswordFile(dataDir); err != nil {
		t.Fatalf("ensurePasswordFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected password file to be created: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected newly created password file to be empty, got %q", data)
	}

	// 2. An existing (non-empty) file must never be clobbered on a later run.
	if err := os.WriteFile(path, []byte("supersecret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensurePasswordFile(dataDir); err != nil {
		t.Fatalf("ensurePasswordFile (second run): %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "supersecret" {
		t.Fatalf("ensurePasswordFile must not overwrite an existing password, got %q, %v", data, err)
	}
}

func TestRoutesInProcess(t *testing.T) {
	dataDir := t.TempDir()
	mux, err := buildMux(dataDir)
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}

	check := func(path, wantCT string, wantMin int) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, wantCT) {
			t.Fatalf("%s: content-type = %q, want contains %q", path, ct, wantCT)
		}
		if rec.Body.Len() < wantMin {
			t.Fatalf("%s: body too small (%d bytes)", path, rec.Body.Len())
		}
		t.Logf("%s -> %s, %d bytes", path, ct, rec.Body.Len())
	}

	check("/", "text/html", 10000)
	check("/?screen=obs", "text/html", 10000)
	check("/style.css", "text/css", 5000)
	check("/fonts/notosans-latin.woff2", "font/woff2", 10000)
	check("/fonts/notosansgurmukhi-gurmukhi.woff2", "font/woff2", 10000)

	// The new responsive/idle-hide classes must have survived the Tailwind
	// rebuild into style.css (they're easy to silently drop if the build
	// script isn't rerun after an index.html edit).
	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	css := rec.Body.String()
	for _, want := range []string{`719px`, `w-\[1em\]`} {
		if !strings.Contains(css, want) {
			t.Errorf("style.css missing expected compiled rule containing %q", want)
		}
	}
}

func TestFindSingleCSV(t *testing.T) {
	dataDir := t.TempDir()

	if _, ok := findSingleCSV(dataDir); ok {
		t.Fatalf("expected no CSV found in an empty directory")
	}

	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte("a,b\n1,2\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("Roster.CSV") // uppercase extension should still count
	if name, ok := findSingleCSV(dataDir); !ok || name != "Roster.CSV" {
		t.Fatalf("expected to find Roster.CSV, got %q, %v", name, ok)
	}

	// A second CSV makes it ambiguous -- must back off, not guess.
	write("Other.csv")
	if _, ok := findSingleCSV(dataDir); ok {
		t.Fatalf("expected ambiguous (2 CSVs) to report not-found")
	}

	// A subdirectory named *.csv must not be mistaken for a file.
	dataDir2 := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir2, "notes.csv"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, ok := findSingleCSV(dataDir2); ok {
		t.Fatalf("a directory named *.csv must not count as a CSV file")
	}
}

func TestHandleAutoCSV(t *testing.T) {
	dataDir := t.TempDir()
	mux, err := buildMux(dataDir)
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}

	get := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/api/auto-csv", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("invalid JSON from /api/auto-csv: %v (%s)", err, rec.Body.String())
		}
		return out
	}

	if out := get(); out["available"] != false {
		t.Fatalf("expected available=false with no CSV present, got %v", out)
	}

	csvContent := "Group,Participant(s)\nA (6-10),Test Singh\n"
	if err := os.WriteFile(filepath.Join(dataDir, "2026 Roster.csv"), []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := get()
	if out["available"] != true {
		t.Fatalf("expected available=true with exactly one CSV present, got %v", out)
	}
	if out["filename"] != "2026 Roster.csv" {
		t.Fatalf("filename = %v, want %q", out["filename"], "2026 Roster.csv")
	}
	if out["content"] != csvContent {
		t.Fatalf("content = %v, want %q", out["content"], csvContent)
	}
}
