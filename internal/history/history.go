package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp time.Time `json:"ts"`
	Provider  string    `json:"provider"`
	Query     string    `json:"query"`
	Command   string    `json:"command"`
	Explain   string    `json:"explain,omitempty"`
}

func Path() string {
	if p := os.Getenv("SPELL_HISTORY"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "spell", "history.jsonl")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "spell", "history.jsonl")
}

// Append writes one entry as a single JSON line. Errors are non-fatal
// and returned so callers can decide whether to surface them.
func Append(e Entry) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = f.Write(b)
	return err
}

// Recent returns up to n latest entries (newest first). Missing file = empty slice.
func Recent(n int) ([]Entry, error) {
	if n <= 0 {
		return nil, nil
	}
	f, err := os.Open(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	// reverse — newest first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}
