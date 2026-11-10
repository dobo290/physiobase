// Package store kapselt alle SQL-Abfragen. Wichtig bei MaxOpenConns(1): nie eine
// zweite Abfrage starten, solange noch Rows offen sind.
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"
)

var ErrNotFound = errors.New("nicht gefunden")

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// ---------- Typen ----------

type Kind struct{ Key, Label, Plural string }

var Kinds = []Kind{
	{"krankheitsbild", "Krankheitsbild", "Krankheitsbilder"},
	{"test", "Test / Assessment", "Tests"},
	{"uebung", "Übung", "Übungen"},
	{"protokoll", "Behandlungsprotokoll", "Protokolle"},
	{"notiz", "Notiz", "Notizen"},
}

func KindLabel(k string) string {
	for _, x := range Kinds {
		if x.Key == k {
			return x.Label
		}
	}
	return k
}

func ValidKind(k string) bool {
	for _, x := range Kinds {
		if x.Key == k {
			return true
		}
	}
	return false
}

type Entry struct {
	ID                                                            int64
	Kind, Title, Summary, Body, Region, Tags, Dosage, PatientText string
	Created, Updated                                              time.Time
}

type Source struct {
	ID, EntryID          int64
	Ref, Page, URL, Note string
}

type Media struct {
	ID, EntryID       int64
	Filename, Caption string
}

type Handout struct {
	ID                      int64
	Title, Recipient, Notes string
	Created, Updated        time.Time
	Count                   int
}

type HandoutItem struct {
	Pos          int
	Dosage, Note string
	Entry        Entry
	Media        []Media
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ---------- Einträge ----------

const entryCols = `e.id, e.kind, e.title, e.summary, e.body, e.region, e.tags, e.dosage, e.patient_text, e.created_at, e.updated_at`

type scanner interface{ Scan(...any) error }

func scanEntry(sc scanner) (Entry, error) {
	var e Entry
	var c, u int64
	err := sc.Scan(&e.ID, &e.Kind, &e.Title, &e.Summary, &e.Body, &e.Region, &e.Tags, &e.Dosage, &e.PatientText, &c, &u)
	e.Created, e.Updated = time.Unix(c, 0), time.Unix(u, 0)
	return e, err
}

func (s *Store) entries(ctx context.Context, q string, args ...any) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FTSQuery macht aus Benutzereingaben eine sichere FTS5-Abfrage: jedes Wort wird
// zum Präfix-Term ("knie" findet "Kniegelenk"), alle Wörter müssen vorkommen.
func FTSQuery(input string) string {
	words := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(words))
	for _, w := range words {
		terms = append(terms, `"`+w+`"*`)
	}
	return strings.Join(terms, " ")
}

// Search durchsucht die Wissensdatenbank. Leere Suche liefert alles (gefiltert).
func (s *Store) Search(ctx context.Context, q, kind, region string) ([]Entry, error) {
	if fts := FTSQuery(q); fts != "" {
		// bm25: Treffer im Titel zählen am meisten
		return s.entries(ctx, `SELECT `+entryCols+` FROM entries_fts f JOIN entries e ON e.id = f.rowid
			WHERE entries_fts MATCH ? AND (? = '' OR e.kind = ?) AND (? = '' OR e.region = ?)
			ORDER BY bm25(entries_fts, 10, 4, 4, 3, 1, 1) LIMIT 500`, fts, kind, kind, region, region)
	}
	return s.entries(ctx, `SELECT `+entryCols+` FROM entries e
		WHERE (? = '' OR e.kind = ?) AND (? = '' OR e.region = ?)
		ORDER BY e.title COLLATE NOCASE LIMIT 500`, kind, kind, region, region)
}

func (s *Store) KindCounts(ctx context.Context) (map[string]int, int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, count(*) FROM entries GROUP BY kind`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, total := map[string]int{}, 0
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, 0, err
		}
		out[k] = n
		total += n
	}
	return out, total, rows.Err()
}

func (s *Store) Regions(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT region FROM entries WHERE region <> '' ORDER BY region COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Entry(ctx context.Context, id int64) (Entry, error) {
	e, err := scanEntry(s.db.QueryRowContext(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.id = ?`, id))
	return e, notFound(err)
}

func (s *Store) CreateEntry(ctx context.Context, e Entry) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO entries (kind, title, summary, body, region, tags, dosage, patient_text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, e.Kind, e.Title, e.Summary, e.Body, e.Region, e.Tags, e.Dosage, e.PatientText)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateEntry(ctx context.Context, e Entry) error {
	res, err := s.db.ExecContext(ctx, `UPDATE entries SET kind=?, title=?, summary=?, body=?, region=?, tags=?,
		dosage=?, patient_text=?, updated_at=unixepoch() WHERE id=?`,
		e.Kind, e.Title, e.Summary, e.Body, e.Region, e.Tags, e.Dosage, e.PatientText, e.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteEntry löscht den Eintrag und gibt die Dateinamen seiner Medien zurück,
// damit der Aufrufer sie von der Platte entfernen kann.
func (s *Store) DeleteEntry(ctx context.Context, id int64) ([]string, error) {
	media, err := s.Media(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM entries WHERE id = ?`, id); err != nil {
		return nil, err
	}
	files := make([]string, len(media))
	for i, m := range media {
		files[i] = m.Filename
	}
	return files, nil
}

func (s *Store) Related(ctx context.Context, id int64) ([]Entry, error) {
	return s.entries(ctx, `SELECT `+entryCols+` FROM entry_links l
		JOIN entries e ON e.id = CASE WHEN l.a = ? THEN l.b ELSE l.a END
		WHERE l.a = ? OR l.b = ? ORDER BY e.kind, e.title COLLATE NOCASE`, id, id, id)
}

// Linkable: alle Einträge außer dem aktuellen und bereits verknüpften.
func (s *Store) Linkable(ctx context.Context, id int64) ([]Entry, error) {
	return s.entries(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.id <> ?
		AND e.id NOT IN (SELECT b FROM entry_links WHERE a = ? UNION SELECT a FROM entry_links WHERE b = ?)
		ORDER BY e.kind, e.title COLLATE NOCASE`, id, id, id)
}

func (s *Store) Link(ctx context.Context, x, y int64) error {
	if x == y {
		return nil
	}
	if x > y {
		x, y = y, x
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO entry_links (a, b) VALUES (?, ?)`, x, y)
	return err
}

func (s *Store) Unlink(ctx context.Context, x, y int64) error {
	if x > y {
		x, y = y, x
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM entry_links WHERE a = ? AND b = ?`, x, y)
	return err
}

// ---------- Quellen ----------

func (s *Store) Sources(ctx context.Context, entryID int64) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entry_id, ref, page, url, note FROM sources WHERE entry_id = ? ORDER BY id`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var x Source
		if err := rows.Scan(&x.ID, &x.EntryID, &x.Ref, &x.Page, &x.URL, &x.Note); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) AddSource(ctx context.Context, x Source) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sources (entry_id, ref, page, url, note) VALUES (?, ?, ?, ?, ?)`,
		x.EntryID, x.Ref, x.Page, x.URL, x.Note)
	return err
}

func (s *Store) DeleteSource(ctx context.Context, id int64) (entryID int64, err error) {
	err = s.db.QueryRowContext(ctx, `DELETE FROM sources WHERE id = ? RETURNING entry_id`, id).Scan(&entryID)
	return entryID, notFound(err)
}

// ---------- Medien ----------

func (s *Store) Media(ctx context.Context, entryID int64) ([]Media, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entry_id, filename, caption FROM media WHERE entry_id = ? ORDER BY id`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Media
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.ID, &m.EntryID, &m.Filename, &m.Caption); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddMedia(ctx context.Context, m Media) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO media (entry_id, filename, caption) VALUES (?, ?, ?)`, m.EntryID, m.Filename, m.Caption)
	return err
}

func (s *Store) DeleteMedia(ctx context.Context, id int64) (Media, error) {
	var m Media
	err := s.db.QueryRowContext(ctx, `DELETE FROM media WHERE id = ? RETURNING id, entry_id, filename, caption`, id).
		Scan(&m.ID, &m.EntryID, &m.Filename, &m.Caption)
	return m, notFound(err)
}

// MediaKnown prüft, ob eine Datei zu einem Eintrag gehört (nur solche werden ausgeliefert).
func (s *Store) MediaKnown(ctx context.Context, filename string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM media WHERE filename = ?`, filename).Scan(&n)
	return n > 0, err
}

// ---------- Handouts ----------

func (s *Store) Handouts(ctx context.Context) ([]Handout, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT h.id, h.title, h.recipient, h.notes, h.created_at, h.updated_at,
		(SELECT count(*) FROM handout_items i WHERE i.handout_id = h.id)
		FROM handouts h ORDER BY h.updated_at DESC, h.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Handout
	for rows.Next() {
		var h Handout
		var c, u int64
		if err := rows.Scan(&h.ID, &h.Title, &h.Recipient, &h.Notes, &c, &u, &h.Count); err != nil {
			return nil, err
		}
		h.Created, h.Updated = time.Unix(c, 0), time.Unix(u, 0)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) Handout(ctx context.Context, id int64) (Handout, error) {
	var h Handout
	var c, u int64
	err := s.db.QueryRowContext(ctx, `SELECT id, title, recipient, notes, created_at, updated_at FROM handouts WHERE id = ?`, id).
		Scan(&h.ID, &h.Title, &h.Recipient, &h.Notes, &c, &u)
	h.Created, h.Updated = time.Unix(c, 0), time.Unix(u, 0)
	return h, notFound(err)
}

func (s *Store) CreateHandout(ctx context.Context, title, recipient string) (int64, error) {
	return s.CreateHandoutWithNotes(ctx, title, recipient, "", 0)
}

// CreateHandoutWithNotes legt ein Handout mit Hinweistext an (pid 0 = ohne Patient).
func (s *Store) CreateHandoutWithNotes(ctx context.Context, title, recipient, notes string, pid int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO handouts (title, recipient, notes, patient_id) VALUES (?, ?, ?, ?)`,
		title, recipient, notes, nullID(pid))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateHandout(ctx context.Context, h Handout) error {
	_, err := s.db.ExecContext(ctx, `UPDATE handouts SET title=?, recipient=?, notes=?, updated_at=unixepoch() WHERE id=?`,
		h.Title, h.Recipient, h.Notes, h.ID)
	return err
}

func (s *Store) DeleteHandout(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM handouts WHERE id = ?`, id)
	return err
}

// CopyHandout dupliziert ein Handout als Vorlage für den nächsten Patienten.
func (s *Store) CopyHandout(ctx context.Context, id int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var nid int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO handouts (title, recipient, notes)
		SELECT title || ' (Kopie)', '', notes FROM handouts WHERE id = ? RETURNING id`, id).Scan(&nid); err != nil {
		return 0, notFound(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO handout_items (handout_id, entry_id, pos, dosage, note)
		SELECT ?, entry_id, pos, dosage, note FROM handout_items WHERE handout_id = ?`, nid, id); err != nil {
		return 0, err
	}
	return nid, tx.Commit()
}

func (s *Store) HandoutItems(ctx context.Context, hid int64) ([]HandoutItem, error) {
	out, err := s.handoutItemRows(ctx, hid)
	if err != nil {
		return nil, err
	}
	// Medien erst nach dem Schließen der Rows laden: nur eine Verbindung
	for i := range out {
		if out[i].Media, err = s.Media(ctx, out[i].Entry.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) handoutItemRows(ctx context.Context, hid int64) ([]HandoutItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.pos, i.dosage, i.note, `+entryCols+`
		FROM handout_items i JOIN entries e ON e.id = i.entry_id
		WHERE i.handout_id = ? ORDER BY i.pos, e.title COLLATE NOCASE`, hid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HandoutItem
	for rows.Next() {
		var it HandoutItem
		e := &it.Entry
		var c, u int64
		if err := rows.Scan(&it.Pos, &it.Dosage, &it.Note, &e.ID, &e.Kind, &e.Title, &e.Summary, &e.Body,
			&e.Region, &e.Tags, &e.Dosage, &e.PatientText, &c, &u); err != nil {
			return nil, err
		}
		e.Created, e.Updated = time.Unix(c, 0), time.Unix(u, 0)
		out = append(out, it)
	}
	return out, rows.Err()
}

// ExercisesNotIn listet Übungen, die noch nicht im Handout sind.
func (s *Store) ExercisesNotIn(ctx context.Context, hid int64) ([]Entry, error) {
	return s.entries(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.kind = 'uebung'
		AND e.id NOT IN (SELECT entry_id FROM handout_items WHERE handout_id = ?)
		ORDER BY e.region COLLATE NOCASE, e.title COLLATE NOCASE`, hid)
}

func (s *Store) AddHandoutItem(ctx context.Context, hid, eid int64) error {
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO handout_items (handout_id, entry_id, pos, dosage)
		SELECT ?, e.id, COALESCE((SELECT max(pos) FROM handout_items WHERE handout_id = ?), 0) + 1, e.dosage
		FROM entries e WHERE e.id = ? AND e.kind = 'uebung'
		AND EXISTS (SELECT 1 FROM handouts WHERE id = ?)`, hid, hid, eid, hid)
	if err != nil {
		return err
	}
	return s.touchHandout(ctx, hid, res)
}

func (s *Store) touchHandout(ctx context.Context, hid int64, res sql.Result) error {
	if n, _ := res.RowsAffected(); n > 0 {
		_, err := s.db.ExecContext(ctx, `UPDATE handouts SET updated_at = unixepoch() WHERE id = ?`, hid)
		return err
	}
	return nil
}

func (s *Store) UpdateHandoutItem(ctx context.Context, hid, eid int64, dosage, note string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE handout_items SET dosage=?, note=? WHERE handout_id=? AND entry_id=?`, dosage, note, hid, eid)
	if err != nil {
		return err
	}
	return s.touchHandout(ctx, hid, res)
}

func (s *Store) RemoveHandoutItem(ctx context.Context, hid, eid int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM handout_items WHERE handout_id=? AND entry_id=?`, hid, eid)
	if err != nil {
		return err
	}
	return s.touchHandout(ctx, hid, res)
}

// MoveHandoutItemUp tauscht die Übung mit ihrer Vorgängerin und nummeriert neu.
func (s *Store) MoveHandoutItemUp(ctx context.Context, hid, eid int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	order, err := handoutOrder(ctx, tx, hid)
	if err != nil {
		return err
	}
	for i := 1; i < len(order); i++ {
		if order[i] == eid {
			order[i-1], order[i] = order[i], order[i-1]
			break
		}
	}
	for i, id := range order {
		if _, err := tx.ExecContext(ctx, `UPDATE handout_items SET pos = ? WHERE handout_id = ? AND entry_id = ?`, i+1, hid, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func handoutOrder(ctx context.Context, tx *sql.Tx, hid int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT i.entry_id FROM handout_items i JOIN entries e ON e.id = i.entry_id
		WHERE i.handout_id = ? ORDER BY i.pos, e.title COLLATE NOCASE`, hid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var order []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		order = append(order, id)
	}
	return order, rows.Err()
}

// ---------- Einstellungen ----------

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
