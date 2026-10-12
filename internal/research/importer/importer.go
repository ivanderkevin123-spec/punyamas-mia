// Package importer parses Markdown, CSV and JSON research notes with mandatory
// provenance and rejects files that look like they contain personal data.
package importer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Item struct {
	Ref       string
	Row       int
	Title     string
	Statement string
	Excerpt   string
	Kind      string
	Quality   string
	TopicKey  string
	Language  string
}

type Note struct {
	File        string
	SourceName  string
	Reference   string
	Publisher   string
	RetrievedAt string
	PublishedAt string
	SourceType  string
	Kind        string
	Quality     string
	Language    string
	Sample      bool
	Items       []Item
}

type FileError struct {
	File    string
	Message string
}

type Result struct {
	Notes   []Note
	Errors  []FileError
	Skipped []string
}

var (
	phoneRe   = regexp.MustCompile(`(?:^|[^0-9A-Za-z])(?:\+62|62|0)8[0-9]{7,11}(?:[^0-9]|$)`)
	nikRe     = regexp.MustCompile(`(?:^|[^0-9])[0-9]{16}(?:[^0-9]|$)`)
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	piiHeader = regexp.MustCompile(`(?i)^(phone|telepon|no_hp|nomor_hp|hp|whatsapp|wa|nik|ktp|no_ktp|email|nama_pelanggan|customer_name|nama_customer|alamat_pelanggan|customer_address)$`)
)

// PII reports whether text contains phone numbers, 16-digit identity numbers or email addresses.
func PII(text string) (bool, string) {
	switch {
	case phoneRe.MatchString(text):
		return true, "nomor telepon"
	case nikRe.MatchString(text):
		return true, "nomor identitas 16 digit"
	case emailRe.MatchString(text):
		return true, "alamat email"
	}
	return false, ""
}

// ImportDir parses every .md, .csv and .json file directly under dir.
func ImportDir(dir string, maxBytes int64) (Result, error) {
	var res Result
	entries, err := os.ReadDir(dir)
	if err != nil {
		return res, fmt.Errorf("direktori impor tidak dapat dibaca: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".md" && ext != ".csv" && ext != ".json" {
			res.Skipped = append(res.Skipped, name)
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			res.Errors = append(res.Errors, FileError{File: name, Message: err.Error()})
			continue
		}
		if info.Size() > maxBytes {
			res.Errors = append(res.Errors, FileError{File: name, Message: fmt.Sprintf("berkas melebihi batas %d byte", maxBytes)})
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			res.Errors = append(res.Errors, FileError{File: name, Message: err.Error()})
			continue
		}
		var notes []Note
		var perr error
		switch ext {
		case ".md":
			notes, perr = ParseMarkdown(name, b)
		case ".csv":
			notes, perr = ParseCSV(name, b)
		case ".json":
			notes, perr = ParseJSON(name, b)
		}
		if perr != nil {
			res.Errors = append(res.Errors, FileError{File: name, Message: perr.Error()})
			continue
		}
		if len(notes) == 0 {
			res.Errors = append(res.Errors, FileError{File: name, Message: "tidak ada butir riset yang dapat dibaca"})
			continue
		}
		res.Notes = append(res.Notes, notes...)
	}
	return res, nil
}

type noteMeta struct {
	SourceName  string `yaml:"source_name" json:"source_name"`
	Reference   string `yaml:"reference" json:"reference"`
	Publisher   string `yaml:"publisher" json:"publisher"`
	RetrievedAt string `yaml:"retrieved_at" json:"retrieved_at"`
	PublishedAt string `yaml:"published_at" json:"published_at"`
	SourceType  string `yaml:"source_type" json:"source_type"`
	Kind        string `yaml:"kind" json:"kind"`
	Quality     string `yaml:"quality" json:"quality"`
	Language    string `yaml:"language" json:"language"`
	Sample      bool   `yaml:"sample" json:"sample"`
}

// ParseMarkdown reads YAML front matter and one item per "## " section.
// Inside a section: "> " lines form the excerpt, "Topik: x" sets the topic,
// "Jenis: fact|hypothesis" and "Kualitas: A-D" override the note defaults.
func ParseMarkdown(name string, b []byte) ([]Note, error) {
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, errors.New("front matter YAML (--- ... ---) wajib ada")
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, errors.New("front matter tidak ditutup dengan ---")
	}
	var meta noteMeta
	dec := yaml.NewDecoder(strings.NewReader(rest[:end]))
	dec.KnownFields(true)
	if err := dec.Decode(&meta); err != nil {
		return nil, fmt.Errorf("front matter: %w", err)
	}
	body := rest[end+4:]
	note := Note{File: name, SourceName: meta.SourceName, Reference: meta.Reference, Publisher: meta.Publisher, RetrievedAt: meta.RetrievedAt, PublishedAt: meta.PublishedAt, SourceType: meta.SourceType, Kind: meta.Kind, Quality: meta.Quality, Language: meta.Language, Sample: meta.Sample}
	var cur *Item
	var stmt, exc []string
	flush := func() {
		if cur == nil {
			return
		}
		cur.Statement = strings.TrimSpace(strings.Join(stmt, " "))
		cur.Excerpt = strings.TrimSpace(strings.Join(exc, " "))
		note.Items = append(note.Items, *cur)
		cur, stmt, exc = nil, nil, nil
	}
	n := 0
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "## "):
			flush()
			n++
			cur = &Item{Row: n, Ref: fmt.Sprintf("%s#%d", name, n), Title: strings.TrimSpace(strings.TrimPrefix(t, "## "))}
		case cur == nil:
			continue
		case strings.HasPrefix(t, ">"):
			exc = append(exc, strings.TrimSpace(strings.TrimPrefix(t, ">")))
		case strings.HasPrefix(t, "Topik:"):
			cur.TopicKey = normalizeTopic(strings.TrimPrefix(t, "Topik:"))
		case strings.HasPrefix(t, "Jenis:"):
			cur.Kind = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "Jenis:")))
		case strings.HasPrefix(t, "Kualitas:"):
			cur.Quality = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(t, "Kualitas:")))
		case t == "":
			continue
		default:
			stmt = append(stmt, t)
		}
	}
	flush()
	if err := finalize(&note); err != nil {
		return nil, err
	}
	return []Note{note}, nil
}

var csvRequired = []string{"source_name", "reference", "retrieved_at", "kind", "title", "statement"}

// ParseCSV reads one item per row; rows sharing source_name and reference form one note.
func ParseCSV(name string, b []byte) ([]Note, error) {
	r := csv.NewReader(bytes.NewReader(b))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("header CSV: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		if piiHeader.MatchString(h) {
			return nil, fmt.Errorf("kolom %q menyerupai data pribadi; berkas ditolak", h)
		}
		idx[h] = i
	}
	for _, req := range csvRequired {
		if _, ok := idx[req]; !ok {
			return nil, fmt.Errorf("kolom wajib %q tidak ada", req)
		}
	}
	get := func(rec []string, col string) string {
		i, ok := idx[col]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	notes := map[string]*Note{}
	var order []string
	row := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("baris %d: %w", row+1, err)
		}
		row++
		key := get(rec, "source_name") + "\x00" + get(rec, "reference")
		n, ok := notes[key]
		if !ok {
			n = &Note{File: name, SourceName: get(rec, "source_name"), Reference: get(rec, "reference"), Publisher: get(rec, "publisher"), RetrievedAt: get(rec, "retrieved_at"), PublishedAt: get(rec, "published_at"), SourceType: get(rec, "source_type"), Language: get(rec, "language")}
			notes[key] = n
			order = append(order, key)
		}
		n.Items = append(n.Items, Item{Row: row, Ref: fmt.Sprintf("%s#%d", name, row), Title: get(rec, "title"), Statement: get(rec, "statement"), Excerpt: get(rec, "excerpt"), Kind: strings.ToLower(get(rec, "kind")), Quality: strings.ToUpper(get(rec, "quality")), TopicKey: normalizeTopic(get(rec, "topic_key")), Language: get(rec, "language")})
	}
	var out []Note
	for _, k := range order {
		n := notes[k]
		if err := finalize(n); err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, nil
}

type jsonItem struct {
	Title     string `json:"title"`
	Statement string `json:"statement"`
	Excerpt   string `json:"excerpt"`
	Kind      string `json:"kind"`
	Quality   string `json:"quality"`
	TopicKey  string `json:"topic_key"`
	Language  string `json:"language"`
}

type jsonNote struct {
	noteMeta
	Items []jsonItem `json:"items"`
}

// ParseJSON reads an array of notes (or a single note object) with embedded items.
func ParseJSON(name string, b []byte) ([]Note, error) {
	trimmed := bytes.TrimSpace(b)
	var raws []jsonNote
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if bytes.HasPrefix(trimmed, []byte("[")) {
		if err := dec.Decode(&raws); err != nil {
			return nil, fmt.Errorf("JSON: %w", err)
		}
	} else {
		var one jsonNote
		if err := dec.Decode(&one); err != nil {
			return nil, fmt.Errorf("JSON: %w", err)
		}
		raws = []jsonNote{one}
	}
	var out []Note
	n := 0
	for _, raw := range raws {
		note := Note{File: name, SourceName: raw.SourceName, Reference: raw.Reference, Publisher: raw.Publisher, RetrievedAt: raw.RetrievedAt, PublishedAt: raw.PublishedAt, SourceType: raw.SourceType, Kind: raw.Kind, Quality: raw.Quality, Language: raw.Language, Sample: raw.Sample}
		for _, it := range raw.Items {
			n++
			note.Items = append(note.Items, Item{Row: n, Ref: fmt.Sprintf("%s#%d", name, n), Title: it.Title, Statement: it.Statement, Excerpt: it.Excerpt, Kind: strings.ToLower(it.Kind), Quality: strings.ToUpper(it.Quality), TopicKey: normalizeTopic(it.TopicKey), Language: it.Language})
		}
		if err := finalize(&note); err != nil {
			return nil, err
		}
		out = append(out, note)
	}
	return out, nil
}

func normalizeTopic(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), "_")
	return s
}

// finalize applies defaults and validates provenance, kinds, qualities, dates and PII.
func finalize(n *Note) error {
	if n.SourceName == "" || n.Reference == "" || n.RetrievedAt == "" {
		return errors.New("source_name, reference dan retrieved_at wajib diisi")
	}
	if err := checkDate(n.RetrievedAt); err != nil {
		return fmt.Errorf("retrieved_at: %w", err)
	}
	if n.PublishedAt != "" {
		if err := checkDate(n.PublishedAt); err != nil {
			return fmt.Errorf("published_at: %w", err)
		}
	}
	if n.SourceType == "" {
		n.SourceType = "imported_file"
	}
	if n.Language == "" {
		n.Language = "id"
	}
	if n.Kind == "" {
		n.Kind = "fact"
	}
	for _, s := range []string{n.SourceName, n.Reference, n.Publisher} {
		if ok, what := PII(s); ok {
			return fmt.Errorf("metadata sumber mengandung %s; berkas ditolak", what)
		}
	}
	if len(n.Items) == 0 {
		return errors.New("tidak ada butir riset")
	}
	for i := range n.Items {
		it := &n.Items[i]
		if it.Kind == "" {
			it.Kind = n.Kind
		}
		if it.Kind != "fact" && it.Kind != "hypothesis" {
			return fmt.Errorf("butir %s: jenis harus fact atau hypothesis", it.Ref)
		}
		if it.Quality == "" {
			it.Quality = n.Quality
		}
		if it.Quality == "" {
			if it.Kind == "fact" {
				it.Quality = "B"
			} else {
				it.Quality = "D"
			}
		}
		if it.Quality < "A" || it.Quality > "D" || len(it.Quality) != 1 {
			return fmt.Errorf("butir %s: kualitas harus A, B, C atau D", it.Ref)
		}
		if it.Language == "" {
			it.Language = n.Language
		}
		if it.Title == "" || it.Statement == "" {
			return fmt.Errorf("butir %s: judul dan pernyataan wajib diisi", it.Ref)
		}
		if it.Kind == "fact" && it.Excerpt == "" {
			return fmt.Errorf("butir %s: fakta wajib menyertakan kutipan bukti (excerpt)", it.Ref)
		}
		if len(it.Statement) > 2000 || len(it.Excerpt) > 1000 {
			return fmt.Errorf("butir %s: pernyataan maksimal 2000 karakter dan kutipan 1000 karakter", it.Ref)
		}
		for _, s := range []string{it.Title, it.Statement, it.Excerpt} {
			if ok, what := PII(s); ok {
				return fmt.Errorf("butir %s mengandung %s; berkas ditolak", it.Ref, what)
			}
		}
	}
	return nil
}

func checkDate(s string) error {
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return nil
	}
	return fmt.Errorf("format tanggal harus YYYY-MM-DD atau RFC 3339, bukan %q", s)
}
