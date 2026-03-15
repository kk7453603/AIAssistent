package httpadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/adapters/http/ui"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type obsidianConfig struct {
	Vaults                 []obsidianVault `json:"vaults"`
	DefaultIntervalMinutes int             `json:"default_interval_minutes"`
}

type obsidianVault struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes *int   `json:"interval_minutes,omitempty"`
}

type obsidianVaultView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes *int   `json:"interval_minutes,omitempty"`
	LastSyncEpoch   *int64 `json:"last_sync_epoch,omitempty"`
	LastStatus      string `json:"last_status,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type obsidianVaultListResponse struct {
	Vaults                 []obsidianVaultView `json:"vaults"`
	DefaultIntervalMinutes int                 `json:"default_interval_minutes"`
}

type obsidianVaultUpsertRequest struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Enabled         *bool  `json:"enabled"`
	IntervalMinutes *int   `json:"interval_minutes"`
}

type obsidianVaultSyncRequest struct {
	WaitReady *bool `json:"wait_ready"`
}

type obsidianSyncError struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type obsidianSyncResult struct {
	Name     string              `json:"name"`
	ID       string              `json:"id"`
	Status   string              `json:"status"`
	Uploaded int                 `json:"uploaded"`
	Skipped  int                 `json:"skipped"`
	Failed   int                 `json:"failed"`
	Errors   []obsidianSyncError `json:"errors,omitempty"`
}

type obsidianSyncResponse struct {
	Results []obsidianSyncResult `json:"results"`
}

type obsidianVaultMeta struct {
	LastSyncEpoch *int64 `json:"last_sync_epoch,omitempty"`
	LastStatus    string `json:"last_status,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

var obsidianSlugRe = regexp.MustCompile(`[^a-z0-9_.-]+`)

func (rt *Router) handleObsidianUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ui/obsidian" {
		http.Redirect(w, r, "/ui/obsidian", http.StatusTemporaryRedirect)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ui.ObsidianHTML)
}

func (rt *Router) handleObsidianList(w http.ResponseWriter, _ *http.Request) {
	cfg, err := rt.loadObsidianConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	vaults := make([]obsidianVaultView, 0, len(cfg.Vaults))
	for _, v := range cfg.Vaults {
		vaultID := v.ID
		if vaultID == "" {
			vaultID = slugifyObsidian(v.Name)
		}
		meta := rt.loadObsidianMeta(vaultID)
		vaults = append(vaults, obsidianVaultView{
			ID:              vaultID,
			Name:            v.Name,
			Path:            v.Path,
			Enabled:         v.Enabled,
			IntervalMinutes: v.IntervalMinutes,
			LastSyncEpoch:   meta.LastSyncEpoch,
			LastStatus:      meta.LastStatus,
			LastError:       meta.LastError,
		})
	}

	resp := obsidianVaultListResponse{
		Vaults:                 vaults,
		DefaultIntervalMinutes: cfg.DefaultIntervalMinutes,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (rt *Router) handleObsidianUpsert(w http.ResponseWriter, r *http.Request) {
	var req obsidianVaultUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path is required"))
		return
	}
	if req.IntervalMinutes != nil && *req.IntervalMinutes < 1 {
		writeError(w, http.StatusBadRequest, errors.New("interval_minutes must be >= 1"))
		return
	}

	resolvedPath, err := rt.resolveVaultPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	vaultID := slugifyObsidian(req.Name)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rt.obsidianMu.Lock()
	defer rt.obsidianMu.Unlock()

	cfg, err := rt.loadObsidianConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	updated := false
	for idx := range cfg.Vaults {
		if cfg.Vaults[idx].ID == vaultID || cfg.Vaults[idx].Name == req.Name {
			cfg.Vaults[idx].ID = vaultID
			cfg.Vaults[idx].Name = req.Name
			cfg.Vaults[idx].Path = resolvedPath
			cfg.Vaults[idx].Enabled = enabled
			if req.IntervalMinutes != nil {
				cfg.Vaults[idx].IntervalMinutes = req.IntervalMinutes
			}
			updated = true
			break
		}
	}

	if !updated {
		cfg.Vaults = append(cfg.Vaults, obsidianVault{
			ID:              vaultID,
			Name:            req.Name,
			Path:            resolvedPath,
			Enabled:         enabled,
			IntervalMinutes: req.IntervalMinutes,
		})
	}

	if err := rt.saveObsidianConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"vault": obsidianVault{
			ID:              vaultID,
			Name:            req.Name,
			Path:            resolvedPath,
			Enabled:         enabled,
			IntervalMinutes: req.IntervalMinutes,
		},
	})
}

func (rt *Router) handleObsidianRemove(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("id is required"))
		return
	}

	rt.obsidianMu.Lock()
	defer rt.obsidianMu.Unlock()

	cfg, err := rt.loadObsidianConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	newVaults := make([]obsidianVault, 0, len(cfg.Vaults))
	removed := false
	for _, v := range cfg.Vaults {
		if v.ID == id || v.Name == id {
			removed = true
			continue
		}
		newVaults = append(newVaults, v)
	}
	if !removed {
		writeError(w, http.StatusNotFound, errors.New("vault not found"))
		return
	}
	cfg.Vaults = newVaults
	if err := rt.saveObsidianConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"removed": id,
	})
}

func (rt *Router) handleObsidianSync(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("id is required"))
		return
	}

	var req obsidianVaultSyncRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	waitReady := true
	if req.WaitReady != nil {
		waitReady = *req.WaitReady
	}

	rt.obsidianMu.Lock()
	cfg, err := rt.loadObsidianConfig()
	rt.obsidianMu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	targets := make([]obsidianVault, 0)
	if id == "all" {
		for _, v := range cfg.Vaults {
			if v.Enabled {
				targets = append(targets, v)
			}
		}
	} else {
		for _, v := range cfg.Vaults {
			if v.ID == id || v.Name == id {
				targets = append(targets, v)
				break
			}
		}
	}

	if len(targets) == 0 {
		writeError(w, http.StatusNotFound, errors.New("vault not found or disabled"))
		return
	}

	results := make([]obsidianSyncResult, 0, len(targets))
	for _, vault := range targets {
		result := rt.syncObsidianVault(r.Context(), vault, waitReady)
		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, obsidianSyncResponse{Results: results})
}

func (rt *Router) syncObsidianVault(ctx context.Context, vault obsidianVault, waitReady bool) obsidianSyncResult {
	vaultID := vault.ID
	if vaultID == "" {
		vaultID = slugifyObsidian(vault.Name)
	}

	path, err := rt.resolveVaultPath(vault.Path)
	if err != nil {
		return rt.failSync(vault, vaultID, err.Error())
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return rt.failSync(vault, vaultID, fmt.Sprintf("vault path not found: %s", path))
	}

	lockPath := rt.statePath(vaultID) + ".lock"
	if err := os.Mkdir(lockPath, 0o755); err != nil {
		return rt.failSync(vault, vaultID, "vault is locked by another sync")
	}
	defer func() { _ = os.Remove(lockPath) }()

	state := rt.loadObsidianState(vaultID)
	rows := make([]obsidianStateRow, 0)
	result := obsidianSyncResult{Name: vault.Name, ID: vaultID, Status: "ok"}

	files, err := listMarkdownFiles(path)
	if err != nil {
		return rt.failSync(vault, vaultID, err.Error())
	}
	for _, filePath := range files {
		rel, err := filepath.Rel(path, filePath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, obsidianSyncError{File: filePath, Error: err.Error()})
			continue
		}

		hash, err := hashFile(filePath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, obsidianSyncError{File: rel, Error: err.Error()})
			prev := state[rel]
			rows = append(rows, obsidianStateRow{RelPath: rel, Hash: prev.Hash, DocumentID: prev.DocumentID})
			continue
		}

		if prev, ok := state[rel]; ok && prev.Hash == hash {
			result.Skipped++
			rows = append(rows, obsidianStateRow{RelPath: rel, Hash: prev.Hash, DocumentID: prev.DocumentID})
			continue
		}

		docID, err := rt.ingestFile(ctx, filePath, waitReady)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, obsidianSyncError{File: rel, Error: err.Error()})
			prev := state[rel]
			rows = append(rows, obsidianStateRow{RelPath: rel, Hash: prev.Hash, DocumentID: prev.DocumentID})
			continue
		}
		result.Uploaded++
		rows = append(rows, obsidianStateRow{RelPath: rel, Hash: hash, DocumentID: docID})
	}

	if err := rt.saveObsidianState(vaultID, rows); err != nil {
		return rt.failSync(vault, vaultID, err.Error())
	}

	status := "ok"
	if result.Failed > 0 {
		status = "partial"
	}
	rt.writeObsidianMeta(vaultID, status, "")
	return result
}

func (rt *Router) ingestFile(ctx context.Context, path string, waitReady bool) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	doc, err := rt.ingestor.Upload(ctx, filepath.Base(path), "text/markdown", file)
	if err != nil {
		return "", err
	}
	if !waitReady {
		return doc.ID, nil
	}

	if _, err := rt.waitDocumentReady(ctx, doc.ID); err != nil {
		return doc.ID, err
	}
	return doc.ID, nil
}

func (rt *Router) waitDocumentReady(ctx context.Context, documentID string) (*domain.Document, error) {
	deadline := time.Now().Add(rt.obsidianSyncTimeout)
	for {
		doc, err := rt.docs.GetByID(ctx, documentID)
		if err != nil {
			return nil, err
		}
		if doc.Status == domain.StatusReady || doc.Status == domain.StatusFailed {
			if doc.Status == domain.StatusFailed {
				return doc, fmt.Errorf("document processing failed: %s", doc.Error)
			}
			return doc, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for document %s", documentID)
		}
		select {
		case <-time.After(rt.obsidianSyncPoll):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type obsidianStateRow struct {
	RelPath    string
	Hash       string
	DocumentID string
}

func (rt *Router) loadObsidianState(vaultID string) map[string]obsidianStateRow {
	state := make(map[string]obsidianStateRow)
	path := rt.statePath(vaultID)
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		row := obsidianStateRow{
			RelPath:    parts[0],
			Hash:       parts[1],
			DocumentID: "",
		}
		if len(parts) > 2 {
			row.DocumentID = parts[2]
		}
		state[row.RelPath] = row
	}
	return state
}

func (rt *Router) saveObsidianState(vaultID string, rows []obsidianStateRow) error {
	path := rt.statePath(vaultID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.RelPath == "" {
			continue
		}
		if _, err := fmt.Fprintf(file, "%s\t%s\t%s\n", row.RelPath, row.Hash, row.DocumentID); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (rt *Router) loadObsidianConfig() (obsidianConfig, error) {
	cfg := obsidianConfig{
		Vaults:                 []obsidianVault{},
		DefaultIntervalMinutes: rt.obsidianDefaultIntervalMinutes,
	}
	if rt.obsidianConfigPath == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(rt.obsidianConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.DefaultIntervalMinutes <= 0 {
		cfg.DefaultIntervalMinutes = rt.obsidianDefaultIntervalMinutes
	}
	if cfg.Vaults == nil {
		cfg.Vaults = []obsidianVault{}
	}
	return cfg, nil
}

func (rt *Router) saveObsidianConfig(cfg obsidianConfig) error {
	if cfg.DefaultIntervalMinutes <= 0 {
		cfg.DefaultIntervalMinutes = rt.obsidianDefaultIntervalMinutes
	}
	if err := os.MkdirAll(filepath.Dir(rt.obsidianConfigPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := rt.obsidianConfigPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, rt.obsidianConfigPath)
}

func (rt *Router) resolveVaultPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(rt.obsidianVaultsRoot, path)
	}
	path = filepath.Clean(path)
	if rt.obsidianVaultsRoot != "" {
		root := filepath.Clean(rt.obsidianVaultsRoot)
		if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return "", fmt.Errorf("path must be under %s", root)
		}
	}
	return path, nil
}

func (rt *Router) loadObsidianMeta(vaultID string) obsidianVaultMeta {
	path := filepath.Join(rt.obsidianStateDir, fmt.Sprintf("%s.meta.json", vaultID))
	data, err := os.ReadFile(path)
	if err != nil {
		return obsidianVaultMeta{}
	}
	var meta obsidianVaultMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return obsidianVaultMeta{}
	}
	return meta
}

func (rt *Router) writeObsidianMeta(vaultID, status, errMsg string) {
	if vaultID == "" {
		return
	}
	if err := os.MkdirAll(rt.obsidianStateDir, 0o755); err != nil {
		return
	}
	now := time.Now().Unix()
	meta := obsidianVaultMeta{
		LastSyncEpoch: &now,
		LastStatus:    status,
		LastError:     errMsg,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(rt.obsidianStateDir, fmt.Sprintf("%s.meta.json", vaultID)), data, 0o644)
}

func (rt *Router) statePath(vaultID string) string {
	return filepath.Join(rt.obsidianStateDir, fmt.Sprintf("%s.tsv", vaultID))
}

func slugifyObsidian(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	trimmed = obsidianSlugRe.ReplaceAllString(trimmed, "_")
	if trimmed == "" {
		return "vault"
	}
	return trimmed
}

func listMarkdownFiles(root string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".obsidian" || name == ".trash" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (rt *Router) failSync(vault obsidianVault, vaultID, errMsg string) obsidianSyncResult {
	rt.writeObsidianMeta(vaultID, "error", errMsg)
	return obsidianSyncResult{
		Name:   vault.Name,
		ID:     vaultID,
		Status: "error",
		Failed: 1,
		Errors: []obsidianSyncError{{File: "", Error: errMsg}},
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
