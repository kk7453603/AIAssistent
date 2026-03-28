package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newObsidianRouter creates a Router configured with temp directories for
// obsidian config, state, and vaults root. Callers can register routes on the
// returned http.ServeMux via the Router's handler methods.
func newObsidianRouter(t *testing.T) (*Router, *http.ServeMux) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "obsidian.json")
	stateDir := filepath.Join(tmpDir, "state")
	vaultsRoot := filepath.Join(tmpDir, "vaults")

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vaultsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	rt := &Router{
		obsidianConfigPath:             configPath,
		obsidianStateDir:               stateDir,
		obsidianVaultsRoot:             vaultsRoot,
		obsidianDefaultIntervalMinutes: 15,
		ingestor:                       ingestSuccessFake{},
		docs:                           docsErrFake{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/obsidian/vaults", rt.handleObsidianList)
	mux.HandleFunc("POST /v1/obsidian/vaults", rt.handleObsidianUpsert)
	mux.HandleFunc("DELETE /v1/obsidian/vaults/{id}", rt.handleObsidianRemove)
	mux.HandleFunc("POST /v1/obsidian/vaults/{id}/notes", rt.handleObsidianCreateNote)
	mux.HandleFunc("GET /v1/obsidian/vaults/{id}/files", rt.handleObsidianListFiles)
	mux.HandleFunc("GET /v1/obsidian/vaults/{id}/files/content", rt.handleObsidianFileContent)

	return rt, mux
}

// writeObsidianConfigFile writes an obsidianConfig JSON to the router's config path.
func writeObsidianConfigFile(t *testing.T, rt *Router, cfg obsidianConfig) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(rt.obsidianConfigPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rt.obsidianConfigPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Unit tests for helper functions ---

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"Hello World", "Hello World"},
		{"file/name", "file_name"},
		{"a\\b", "a_b"},
		{"co:lon", "co_lon"},
		{"star*", "star_"},
		{"q?mark", "q_mark"},
		{`"quoted"`, `_quoted_`},
		{"a<b>c", "a_b_c"},
		{"pipe|line", "pipe_line"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{strings.Repeat("a", 250), strings.Repeat("a", 200)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafePath(t *testing.T) {
	root := "/vault/root"
	tests := []struct {
		name    string
		rel     string
		wantErr bool
		want    string
	}{
		{"empty returns root", "", false, "/vault/root"},
		{"simple subdir", "notes", false, "/vault/root/notes"},
		{"nested path", "a/b/c.md", false, "/vault/root/a/b/c.md"},
		{"traversal blocked", "../../../etc/passwd", true, ""},
		{"absolute path rejected", "/etc/passwd", true, ""},
		{"dot-dot in middle", "a/../../etc", true, ""},
		{"current dir", ".", false, "/vault/root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safePath(root, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Errorf("safePath(%q, %q) expected error, got %q", root, tt.rel, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("safePath(%q, %q) unexpected error: %v", root, tt.rel, err)
			}
			if got != tt.want {
				t.Errorf("safePath(%q, %q) = %q, want %q", root, tt.rel, got, tt.want)
			}
		})
	}
}

func TestSlugifyObsidian(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Vault", "my_vault"},
		{"simple", "simple"},
		{"UPPER", "upper"},
		{"  spaces  ", "spaces"},
		{"special!@#$chars", "special_chars"},
		{"a.b-c_d", "a.b-c_d"},
		{"", "vault"},
		{"   ", "vault"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugifyObsidian(tt.input)
			if got != tt.want {
				t.Errorf("slugifyObsidian(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Handler tests ---

func TestHandleObsidianList_Empty(t *testing.T) {
	_, mux := newObsidianRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp obsidianVaultListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Vaults) != 0 {
		t.Errorf("expected 0 vaults, got %d", len(resp.Vaults))
	}
	if resp.DefaultIntervalMinutes != 15 {
		t.Errorf("expected default_interval_minutes=15, got %d", resp.DefaultIntervalMinutes)
	}
}

func TestHandleObsidianList_WithVaults(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultPath := filepath.Join(rt.obsidianVaultsRoot, "testvault")
	if err := os.MkdirAll(vaultPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "testvault", Name: "Test Vault", Path: vaultPath, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp obsidianVaultListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("expected 1 vault, got %d", len(resp.Vaults))
	}
	if resp.Vaults[0].ID != "testvault" {
		t.Errorf("expected vault ID 'testvault', got %q", resp.Vaults[0].ID)
	}
	if resp.Vaults[0].Name != "Test Vault" {
		t.Errorf("expected vault Name 'Test Vault', got %q", resp.Vaults[0].Name)
	}
}

func TestHandleObsidianUpsert_CreateNew(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	// Create a vault directory so resolveVaultPath succeeds.
	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "newvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"New Vault","path":"` + vaultDir + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}

	// Verify the config file was created.
	cfg, err := rt.loadObsidianConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Vaults) != 1 {
		t.Fatalf("expected 1 vault in config, got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].Name != "New Vault" {
		t.Errorf("expected vault name 'New Vault', got %q", cfg.Vaults[0].Name)
	}
}

func TestHandleObsidianUpsert_MissingName(t *testing.T) {
	_, mux := newObsidianRouter(t)

	body := `{"path":"/some/path"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(errMsg, "name") {
		t.Errorf("expected error about 'name', got %q", errMsg)
	}
}

func TestHandleObsidianUpsert_MissingPath(t *testing.T) {
	_, mux := newObsidianRouter(t)

	body := `{"name":"Vault"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianRemove_Success(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "myvault", Name: "My Vault", Path: "/tmp/v", Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/obsidian/vaults/myvault", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}
	if resp["removed"] != "myvault" {
		t.Errorf("expected removed=myvault, got %v", resp["removed"])
	}

	// Verify vault was removed from config.
	cfg, err := rt.loadObsidianConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Vaults) != 0 {
		t.Errorf("expected 0 vaults after removal, got %d", len(cfg.Vaults))
	}
}

func TestHandleObsidianRemove_NotFound(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	// Empty config, no vaults.
	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults:                 []obsidianVault{},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/obsidian/vaults/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianCreateNote_Success(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "notevault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "notevault", Name: "Note Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	body := `{"title":"Test Note","content":"Hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults/notevault/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "created" {
		t.Errorf("expected status=created, got %v", resp["status"])
	}
	if resp["title"] != "Test Note" {
		t.Errorf("expected title='Test Note', got %v", resp["title"])
	}

	// Verify the file exists on disk.
	notePath := filepath.Join(vaultDir, "Test Note.md")
	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("expected note file at %s: %v", notePath, err)
	}
	if string(data) != "Hello world" {
		t.Errorf("expected file content 'Hello world', got %q", string(data))
	}
}

func TestHandleObsidianCreateNote_WithFolder(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "foldervault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "foldervault", Name: "Folder Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	body := `{"title":"Sub Note","content":"In subfolder","folder":"sub/dir"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults/foldervault/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	notePath := filepath.Join(vaultDir, "sub", "dir", "Sub Note.md")
	if _, err := os.Stat(notePath); err != nil {
		t.Fatalf("expected note file at %s: %v", notePath, err)
	}
}

func TestHandleObsidianCreateNote_MissingFields(t *testing.T) {
	_, mux := newObsidianRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing title", `{"content":"stuff"}`},
		{"missing content", `{"title":"stuff"}`},
		{"empty title", `{"title":"","content":"stuff"}`},
		{"empty content", `{"title":"stuff","content":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a valid vault ID — the handler checks fields before vault lookup for title/content.
			req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults/someid/notes", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleObsidianCreateNote_VaultNotFound(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults:                 []obsidianVault{},
		DefaultIntervalMinutes: 15,
	})

	body := `{"title":"Note","content":"body"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/obsidian/vaults/nope/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianListFiles(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "listvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create some files.
	for _, name := range []string{"alpha.md", "beta.md", "gamma.txt"} {
		if err := os.WriteFile(filepath.Join(vaultDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Create a subdirectory.
	if err := os.MkdirAll(filepath.Join(vaultDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a hidden dir that should be excluded.
	if err := os.MkdirAll(filepath.Join(vaultDir, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "listvault", Name: "List Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/listvault/files", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []obsidianFileEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}

	// Expect subdir (dir first), then alpha.md, beta.md, gamma.txt — no .obsidian.
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	// First entry should be the directory.
	if !entries[0].IsDir || entries[0].Name != "subdir" {
		t.Errorf("expected first entry to be subdir dir, got %+v", entries[0])
	}
	// Remaining should be files sorted alphabetically.
	expectedFiles := []string{"alpha.md", "beta.md", "gamma.txt"}
	for i, want := range expectedFiles {
		if entries[i+1].Name != want {
			t.Errorf("entry[%d]: expected %q, got %q", i+1, want, entries[i+1].Name)
		}
	}
}

func TestHandleObsidianListFiles_VaultNotFound(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults:                 []obsidianVault{},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/nope/files", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianFileContent(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "contentvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fileContent := "# Hello\n\nThis is a test note."
	if err := os.WriteFile(filepath.Join(vaultDir, "note.md"), []byte(fileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "contentvault", Name: "Content Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/contentvault/files/content?path=note.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["content"] != fileContent {
		t.Errorf("expected content %q, got %q", fileContent, resp["content"])
	}
	if resp["path"] != "note.md" {
		t.Errorf("expected path 'note.md', got %q", resp["path"])
	}
}

func TestHandleObsidianFileContent_PathTraversal(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "secvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "secvault", Name: "Sec Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/secvault/files/content?path=../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianFileContent_MissingPath(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "mpvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "mpvault", Name: "MP Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/mpvault/files/content", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObsidianFileContent_FileNotFound(t *testing.T) {
	rt, mux := newObsidianRouter(t)

	vaultDir := filepath.Join(rt.obsidianVaultsRoot, "fnfvault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeObsidianConfigFile(t, rt, obsidianConfig{
		Vaults: []obsidianVault{
			{ID: "fnfvault", Name: "FNF Vault", Path: vaultDir, Enabled: true},
		},
		DefaultIntervalMinutes: 15,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/obsidian/vaults/fnfvault/files/content?path=nonexistent.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"key": "value"})

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["key"] != "value" {
		t.Errorf("expected key=value, got %q", resp["key"])
	}
}
