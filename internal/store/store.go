// Package store holds client settings — everything that is a preference
// rather than money.
//
// The wallet lives in its own file, owned by bank.JSONLedger. Keeping the two
// apart means a settings write can never corrupt a balance, and neither file
// has two writers.
package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
)

// Version is the settings format.
const Version = 1

// DefaultServer is where the Online menu item looks for a table.
const DefaultServer = "felt.local:2222"

// Settings are the client's preferences.
type Settings struct {
	Version int    `json:"version"`
	Server  string `json:"server"`
	Nick    string `json:"nick"`

	// Glyphs picks the slot symbol set: "ascii" or "emoji". ASCII is the
	// default because emoji width varies between terminals and shears the
	// reel layout.
	Glyphs string `json:"glyphs"`
}

// Dir is the configuration directory, created on demand.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "felt"), nil
}

// SettingsPath is the settings file.
func SettingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// SavePath is the wallet file, which bank.JSONLedger owns.
func SavePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "save.json"), nil
}

// Default returns settings for a first run.
func Default() Settings {
	return Settings{
		Version: Version,
		Server:  DefaultServer,
		Nick:    defaultNick(),
		Glyphs:  "ascii",
	}
}

// defaultNick seeds the online nickname from the local account name, which is
// what ssh would have used anyway.
func defaultNick() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if n := os.Getenv("USER"); n != "" {
		return n
	}
	return "guest"
}

// Load reads the settings. A missing or unreadable file yields defaults
// alongside the error, so the client can always start.
func Load() (Settings, error) {
	def := Default()

	path, err := SettingsPath()
	if err != nil {
		return def, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return def, nil
		}
		return def, err
	}

	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return def, err
	}
	return s.normalized(), nil
}

func (s Settings) normalized() Settings {
	def := Default()
	if s.Version == 0 {
		s.Version = Version
	}
	if s.Server == "" {
		s.Server = def.Server
	}
	if s.Nick == "" {
		s.Nick = def.Nick
	}
	if s.Glyphs != "emoji" {
		s.Glyphs = "ascii"
	}
	return s
}

// Save writes the settings atomically.
func (s Settings) Save() error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	s.Version = Version
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(dir, "settings-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Reset deletes both files, for the --reset flag.
func Reset() error {
	for _, p := range []func() (string, error){SettingsPath, SavePath} {
		path, err := p()
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
