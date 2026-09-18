// Package library stores web-only metadata, page text, snapshots and job policies.
package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB   *sql.DB
	Root string
}
type Metadata struct {
	Tags     []string `json:"tags"`
	Favorite bool     `json:"favorite"`
}
type Page struct {
	Number int    `json:"page"`
	Text   string `json:"text"`
	Source string `json:"source"`
}
type Hit struct {
	DocumentID string `json:"documentId"`
	Page       int    `json:"page"`
	Snippet    string `json:"snippet"`
	Source     string `json:"source"`
}
type Snapshot struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Created    string `json:"created"`
	Size       int64  `json:"size"`
	HasPDF     bool   `json:"hasPdf"`
}
type Policy struct {
	AutoIndex     bool   `json:"autoIndex"`
	Snapshots     bool   `json:"snapshots"`
	Retain        int    `json:"retain"`
	BackupEnabled bool   `json:"backupEnabled"`
	Schedule      string `json:"schedule"`
	Format        string `json:"format"`
	Destination   string `json:"destination"`
	Folder        string `json:"folder"`
}
type Run struct {
	ID        int64    `json:"id"`
	Kind      string   `json:"kind"`
	Started   string   `json:"started"`
	Finished  string   `json:"finished"`
	Completed int      `json:"completed"`
	Failures  []string `json:"failures"`
	Status    string   `json:"status"`
}

func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "library.sqlite"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	schema := `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS metadata(uid TEXT NOT NULL,doc TEXT NOT NULL,tags TEXT NOT NULL DEFAULT '[]',favorite INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(uid,doc));
 CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(uid UNINDEXED,doc UNINDEXED,page UNINDEXED,text,source UNINDEXED,tokenize='unicode61');
 CREATE TABLE IF NOT EXISTS indexed(uid TEXT NOT NULL,doc TEXT NOT NULL,version TEXT NOT NULL,PRIMARY KEY(uid,doc));
 CREATE TABLE IF NOT EXISTS snapshots(uid TEXT NOT NULL,id TEXT NOT NULL,doc TEXT NOT NULL,name TEXT NOT NULL,version TEXT NOT NULL,created TEXT NOT NULL,size INTEGER NOT NULL,pdf INTEGER NOT NULL,PRIMARY KEY(uid,id));
 CREATE INDEX IF NOT EXISTS snapshots_doc ON snapshots(uid,doc,created);
 CREATE TABLE IF NOT EXISTS policies(uid TEXT PRIMARY KEY,settings TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS runs(id INTEGER PRIMARY KEY AUTOINCREMENT,uid TEXT NOT NULL,kind TEXT NOT NULL,started TEXT NOT NULL,finished TEXT NOT NULL DEFAULT '',completed INTEGER NOT NULL DEFAULT 0,failures TEXT NOT NULL DEFAULT '[]',status TEXT NOT NULL DEFAULT 'running');
 CREATE TABLE IF NOT EXISTS exported(uid TEXT NOT NULL,destination TEXT NOT NULL,doc TEXT NOT NULL,version TEXT NOT NULL,PRIMARY KEY(uid,destination,doc));
 UPDATE runs SET status='interrupted',finished=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE status='running';`
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err = os.Chmod(filepath.Join(root, "library.sqlite"), 0600); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, Root: root}, nil
}
func (s *Store) Metadata(uid string) (map[string]Metadata, error) {
	rows, err := s.DB.Query("SELECT doc,tags,favorite FROM metadata WHERE uid=?", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Metadata{}
	for rows.Next() {
		var id, tags string
		var m Metadata
		if err = rows.Scan(&id, &tags, &m.Favorite); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(tags), &m.Tags); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}
func (s *Store) SetMetadata(uid, doc string, m Metadata) error {
	if len(m.Tags) > 30 {
		return errors.New("at most 30 tags per document")
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range m.Tags {
		tag = strings.TrimSpace(tag)
		if len(tag) > 60 {
			return errors.New("tags must be at most 60 bytes")
		}
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			seen[key] = true
			tags = append(tags, tag)
		}
	}
	b, _ := json.Marshal(tags)
	_, err := s.DB.Exec("INSERT INTO metadata(uid,doc,tags,favorite) VALUES(?,?,?,?) ON CONFLICT(uid,doc) DO UPDATE SET tags=excluded.tags,favorite=excluded.favorite", uid, doc, string(b), m.Favorite)
	return err
}
func (s *Store) Index(uid, doc, version string, pages []Page) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM pages WHERE uid=? AND doc=?", uid, doc); err != nil {
		return err
	}
	for _, p := range pages {
		if _, err = tx.Exec("INSERT INTO pages(uid,doc,page,text,source) VALUES(?,?,?,?,?)", uid, doc, p.Number, p.Text, p.Source); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO indexed VALUES(?,?,?) ON CONFLICT(uid,doc) DO UPDATE SET version=excluded.version", uid, doc, version); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) IndexedVersion(uid, doc string) string {
	var version string
	s.DB.QueryRow("SELECT version FROM indexed WHERE uid=? AND doc=?", uid, doc).Scan(&version)
	return version
}
func (s *Store) Search(uid, query string) ([]Hit, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return []Hit{}, nil
	}
	if len(query) > 300 || len(words) > 20 {
		return nil, errors.New("search is too long")
	}
	for i, word := range words {
		words[i] = `"` + strings.ReplaceAll(word, `"`, `""`) + `"`
	}
	expression := strings.Join(words, " AND ")
	rows, err := s.DB.Query("SELECT doc,page,snippet(pages,3,'','',' … ',24),source FROM pages WHERE pages MATCH ? AND uid=? ORDER BY rank LIMIT 100", expression, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []Hit{}
	for rows.Next() {
		var hit Hit
		if err = rows.Scan(&hit.DocumentID, &hit.Page, &hit.Snippet, &hit.Source); err != nil {
			return nil, err
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}
func DefaultPolicy() Policy {
	return Policy{Retain: 20, Schedule: "daily", Format: "pdf", Destination: "local"}
}
func (s *Store) Policy(uid string) (Policy, error) {
	p := DefaultPolicy()
	var data string
	err := s.DB.QueryRow("SELECT settings FROM policies WHERE uid=?", uid).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(data), &p)
	return p, err
}
func (s *Store) SetPolicy(uid string, p Policy) error {
	if p.Retain < 1 || p.Retain > 100 {
		return errors.New("retain must be between 1 and 100")
	}
	if p.Schedule != "daily" && p.Schedule != "weekly" {
		return errors.New("schedule must be daily or weekly")
	}
	if p.Format != "pdf" && p.Format != "rmdoc" {
		return errors.New("format must be pdf or rmdoc")
	}
	if p.Destination == "" {
		return errors.New("choose a destination")
	}
	if len(p.Folder) > 500 || strings.Contains(p.Folder, "\x00") {
		return errors.New("invalid destination folder")
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO policies VALUES(?,?) ON CONFLICT(uid) DO UPDATE SET settings=excluded.settings", uid, string(data))
	return err
}
func (s *Store) AddSnapshot(uid string, v Snapshot) error {
	_, err := s.DB.Exec("INSERT INTO snapshots VALUES(?,?,?,?,?,?,?,?)", uid, v.ID, v.DocumentID, v.Name, v.Version, v.Created, v.Size, v.HasPDF)
	return err
}
func (s *Store) Snapshots(uid, doc string) ([]Snapshot, error) {
	rows, err := s.DB.Query("SELECT id,doc,name,version,created,size,pdf FROM snapshots WHERE uid=? AND doc=? ORDER BY created DESC", uid, doc)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var v Snapshot
		if err = rows.Scan(&v.ID, &v.DocumentID, &v.Name, &v.Version, &v.Created, &v.Size, &v.HasPDF); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Snapshot(uid, id string) (Snapshot, error) {
	var v Snapshot
	err := s.DB.QueryRow("SELECT id,doc,name,version,created,size,pdf FROM snapshots WHERE uid=? AND id=?", uid, id).Scan(&v.ID, &v.DocumentID, &v.Name, &v.Version, &v.Created, &v.Size, &v.HasPDF)
	return v, err
}
func (s *Store) DeleteSnapshot(uid, id string) error {
	_, err := s.DB.Exec("DELETE FROM snapshots WHERE uid=? AND id=?", uid, id)
	return err
}
func (s *Store) StartRun(uid, kind string) (int64, error) {
	r, err := s.DB.Exec("INSERT INTO runs(uid,kind,started) VALUES(?,?,?)", uid, kind, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}
func (s *Store) FinishRun(id int64, completed int, failures []string) error {
	if failures == nil {
		failures = []string{}
	}
	data, _ := json.Marshal(failures)
	status := "complete"
	if len(failures) > 0 {
		status = "failed"
	}
	_, err := s.DB.Exec("UPDATE runs SET finished=?,completed=?,failures=?,status=? WHERE id=?", time.Now().UTC().Format(time.RFC3339), completed, string(data), status, id)
	return err
}
func (s *Store) Runs(uid string) ([]Run, error) {
	rows, err := s.DB.Query("SELECT id,kind,started,finished,completed,failures,status FROM runs WHERE uid=? ORDER BY id DESC LIMIT 30", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var r Run
		var failures string
		if err = rows.Scan(&r.ID, &r.Kind, &r.Started, &r.Finished, &r.Completed, &failures, &r.Status); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(failures), &r.Failures); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) Exported(uid, destination, doc, version string) bool {
	var existing string
	err := s.DB.QueryRow("SELECT version FROM exported WHERE uid=? AND destination=? AND doc=?", uid, destination, doc).Scan(&existing)
	return err == nil && version == existing
}
func (s *Store) MarkExported(uid, destination, doc, version string) error {
	_, err := s.DB.Exec("INSERT INTO exported VALUES(?,?,?,?) ON CONFLICT(uid,destination,doc) DO UPDATE SET version=excluded.version", uid, destination, doc, version)
	return err
}
func (s *Store) PruneIndex(uid string, live map[string]bool) error {
	rows, err := s.DB.Query("SELECT doc FROM indexed WHERE uid=?", uid)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !live[id] {
			stale = append(stale, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range stale {
		if _, err = s.DB.Exec("DELETE FROM pages WHERE uid=? AND doc=?", uid, id); err != nil {
			return err
		}
		if _, err = s.DB.Exec("DELETE FROM indexed WHERE uid=? AND doc=?", uid, id); err != nil {
			return err
		}
	}
	return nil
}
func ValidateKind(kind string) error {
	switch kind {
	case "index", "snapshots", "backup":
		return nil
	}
	return fmt.Errorf("unknown job %q", kind)
}
func (s *Store) LastStarted(uid, kind string) string {
	var result string
	s.DB.QueryRow("SELECT started FROM runs WHERE uid=? AND kind=? ORDER BY id DESC LIMIT 1", uid, kind).Scan(&result)
	return result
}

// LatestRun looks beyond the recent activity list, which can be dominated by indexing.
func (s *Store) LatestRun(uid, kind string) (*Run, error) {
	var r Run
	var failures string
	err := s.DB.QueryRow("SELECT id,kind,started,finished,completed,failures,status FROM runs WHERE uid=? AND kind=? ORDER BY id DESC LIMIT 1", uid, kind).Scan(&r.ID, &r.Kind, &r.Started, &r.Finished, &r.Completed, &failures, &r.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(failures), &r.Failures); err != nil {
		return nil, err
	}
	return &r, nil
}
