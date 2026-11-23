package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// agy keeps its login in the system keyring, and only falls back to this file (in
// the agy directory) when no keyring is available. Profiles always use this file name.
const (
	tokenFile       = "antigravity-oauth-token"
	keyringService  = "gemini"
	keyringUser     = "antigravity"
	maxBackups      = 10
	timestampLayout = "2006-01-02T15:04:05-07:00"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ---------------------------------------------------------------- paths

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

func expandUser(p string) string {
	if p == "~" {
		return homeDir()
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(homeDir(), p[2:])
	}
	return p
}

func envPath(name string, def ...string) string {
	if v := os.Getenv(name); v != "" {
		return expandUser(v)
	}
	return filepath.Join(append([]string{homeDir()}, def...)...)
}

func agyHome() string     { return envPath("AGY_HOME", ".gemini", "antigravity-cli") }
func storeHome() string   { return envPath("AGY_SWITCH_HOME", ".agy-switch") }
func profilesDir() string { return filepath.Join(storeHome(), "profiles") }
func stateFile() string   { return filepath.Join(storeHome(), "state.json") }
func liveTokenFile() string {
	return filepath.Join(agyHome(), tokenFile)
}
func profileDir(name string) string { return filepath.Join(profilesDir(), name) }
func tokenPath(name string) string  { return filepath.Join(profileDir(name), tokenFile) }

// ---------------------------------------------------------------- helpers

func validateName(name string) (string, error) {
	if !nameRE.MatchString(name) {
		return "", fmt.Errorf("invalid profile name: %q (use letters, digits, '.', '_' or '-')", name)
	}
	return name, nil
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func ensurePrivateDir(p string) error {
	if err := os.MkdirAll(p, 0o700); err != nil {
		return err
	}
	return os.Chmod(p, 0o700)
}

// atomicWrite writes data to dst with 0600 permissions, replacing it atomically.
func atomicWrite(dst string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o600)
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

func writeJSON(p string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(p, append(data, '\n'))
}

func readFileOrNil(p string) ([]byte, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func removeIfExists(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func now() string { return time.Now().UTC().Format(timestampLayout) }

// ---------------------------------------------------------------- tokens

// emailFromToken returns the account email stored in an agy token, or "".
func emailFromToken(data []byte) string {
	var token map[string]any
	if len(data) == 0 || json.Unmarshal(data, &token) != nil {
		return ""
	}
	if e, ok := token["id_token_email"].(string); ok && e != "" {
		return e
	}
	// Some agy versions (e.g. on Windows) only keep the raw id_token JWT.
	idToken, ok := token["id_token"].(string)
	if !ok || strings.Count(idToken, ".") != 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.Split(idToken, ".")[1], "="))
	if err != nil {
		return ""
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	e, _ := claims["email"].(string)
	return e
}

func tokenEmail(p string) string {
	data, _ := os.ReadFile(p)
	return emailFromToken(data)
}

func profileEmail(name string) string { return tokenEmail(tokenPath(name)) }

func liveEmail() string {
	data, _ := readLiveToken()
	return emailFromToken(data)
}

func readTokenFile() ([]byte, error) { return readFileOrNil(liveTokenFile()) }

// ---------------------------------------------------------------- state

type state struct {
	Active *string `json:"active"`
}

func loadState() state {
	var st state
	if data, err := os.ReadFile(stateFile()); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

func saveState(st state) error {
	if err := ensurePrivateDir(storeHome()); err != nil {
		return err
	}
	return writeJSON(stateFile(), st)
}

func activeProfile() string {
	st := loadState()
	if st.Active != nil && *st.Active != "" && isDir(profileDir(*st.Active)) {
		return *st.Active
	}
	return ""
}

func listProfiles() []string {
	entries, err := os.ReadDir(profilesDir())
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && isFile(tokenPath(e.Name())) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------- core

// snapshotLive copies the current agy credentials into profile name.
func snapshotLive(name string) error {
	data, err := readLiveToken()
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("no login found in the %s or at %s. "+
			"Log in to agy first (or use `agy-switch add NAME --login`).",
			credentialStoreName, liveTokenFile())
	}
	if err := ensurePrivateDir(profilesDir()); err != nil {
		return err
	}
	dir := profileDir(name)
	if err := ensurePrivateDir(dir); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(dir, tokenFile), data); err != nil {
		return err
	}
	metaPath := filepath.Join(dir, "meta.json")
	meta := map[string]any{}
	if raw, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(raw, &meta)
		if meta == nil {
			meta = map[string]any{}
		}
	}
	if _, ok := meta["created_at"]; !ok {
		meta["created_at"] = now()
	}
	meta["updated_at"] = now()
	if e := emailFromToken(data); e != "" {
		meta["email"] = e
	} else {
		meta["email"] = nil
	}
	return writeJSON(metaPath, meta)
}

// syncActiveBack saves agy's newest (self-refreshed) token back into the active
// profile, but only if the live login is still the same account.
func syncActiveBack() error {
	active := activeProfile()
	if active == "" {
		return nil
	}
	if live := liveEmail(); live != "" && live == profileEmail(active) {
		return snapshotLive(active)
	}
	return nil
}

func restoreProfile(name string) error {
	data, err := os.ReadFile(tokenPath(name))
	if err != nil {
		return err
	}
	return writeLiveToken(data)
}

// backupLive saves the current login to the backups directory and returns its
// path, or "" if there is no login to back up.
func backupLive() (string, error) {
	data, err := readLiveToken()
	if err != nil || data == nil {
		return "", err
	}
	dir := filepath.Join(storeHome(), "backups")
	if err := ensurePrivateDir(dir); err != nil {
		return "", err
	}
	t := time.Now()
	stamp := fmt.Sprintf("%s-%06d", t.Format("20060102-150405"), t.Nanosecond()/1000)
	dst := filepath.Join(dir, tokenFile+"."+stamp)
	if err := atomicWrite(dst, data); err != nil {
		return "", err
	}
	old, _ := filepath.Glob(filepath.Join(dir, tokenFile+".*"))
	sort.Strings(old)
	if len(old) > maxBackups {
		for _, p := range old[:len(old)-maxBackups] {
			os.Remove(p)
		}
	}
	return dst, nil
}
