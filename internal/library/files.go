package library

import (
	"archive/zip"
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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/unidoc/unipdf/v3/extractor"
	pdf "github.com/unidoc/unipdf/v3/model"
)

const MaxDocumentBytes int64 = 512 << 20

func AccountKey(uid string) string {
	sum := sha256.Sum256([]byte(uid))
	return hex.EncodeToString(sum[:])
}
func (s *Store) SnapshotPath(uid, id, ext string) string {
	return filepath.Join(s.Root, "snapshots", AccountKey(uid), id+"."+ext)
}

// WriteFile never publishes partial output. A failed render leaves the last good file intact.
func WriteFile(destination string, reader io.Reader) (size int64, err error) {
	if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".pending-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	size, err = io.Copy(f, io.LimitReader(reader, MaxDocumentBytes+1))
	if err != nil {
		return
	}
	if size > MaxDocumentBytes {
		return 0, errors.New("document exceeds 512 MiB processing limit")
	}
	if size == 0 {
		return 0, errors.New("document export was empty")
	}
	if err = f.Sync(); err != nil {
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	err = os.Rename(f.Name(), destination)
	return
}
func ExtractPDF(filename string) ([]Page, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader, err := pdf.NewPdfReader(f)
	if err != nil {
		return nil, err
	}
	count, err := reader.GetNumPages()
	if err != nil {
		return nil, err
	}
	if count > 2000 {
		return nil, errors.New("document exceeds 2000 page indexing limit")
	}
	pages := make([]Page, 0, count)
	for number := 1; number <= count; number++ {
		p, err := reader.GetPage(number)
		if err != nil {
			return nil, err
		}
		ex, err := extractor.New(p)
		if err != nil {
			return nil, err
		}
		text, err := ex.ExtractText()
		if err != nil {
			return nil, err
		}
		pages = append(pages, Page{Number: number, Text: text, Source: "pdf"})
	}
	return pages, nil
}

// OCR sends PDF bytes only to an administrator-configured self-hosted service.
// The service returns {"pages":[{"page":1,"text":"recognized text"}]}.
func OCR(ctx context.Context, endpoint, filename string) ([]Page, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, f)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/pdf")
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OCR redirects are not allowed") }}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR service returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Pages []Page `json:"pages"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Pages) > 2000 {
		return nil, errors.New("too many OCR pages")
	}
	seen := map[int]bool{}
	for i, p := range result.Pages {
		if p.Number < 1 || p.Number > 2000 || seen[p.Number] || len(p.Text) > 1<<20 {
			return nil, errors.New("invalid OCR page")
		}
		seen[p.Number] = true
		result.Pages[i].Source = "ocr"
	}
	return result.Pages, nil
}
func MergePages(printed, recognized []Page) []Page {
	result := append([]Page(nil), printed...)
	positions := map[int]int{}
	for i, p := range result {
		positions[p.Number] = i
	}
	for _, p := range recognized {
		if i, ok := positions[p.Number]; ok {
			if strings.TrimSpace(p.Text) != "" {
				result[i].Text += "\n" + p.Text
				result[i].Source = "pdf+ocr"
			}
		} else {
			positions[p.Number] = len(result)
			result = append(result, p)
		}
	}
	return result
}

// CloneArchive assigns a fresh UUID so restoration never overwrites a live notebook.
func CloneArchive(filename, name, parent string) (*os.File, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	var original string
	for _, entry := range archive.File {
		if strings.HasSuffix(entry.Name, ".metadata") {
			if original != "" {
				return nil, errors.New("archive contains multiple documents")
			}
			original = strings.TrimSuffix(entry.Name, ".metadata")
		}
	}
	if original == "" || strings.ContainsAny(original, "/\\") {
		return nil, errors.New("invalid snapshot metadata")
	}
	id := uuid.NewString()
	output, err := os.CreateTemp("", "rm-library-restore-*.rmdoc")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			output.Close()
			os.Remove(output.Name())
		}
	}()
	writer := zip.NewWriter(output)
	var total uint64
	for _, entry := range archive.File {
		total += entry.UncompressedSize64
		if total > uint64(MaxDocumentBytes) {
			return nil, errors.New("snapshot exceeds restore limit")
		}
		if entry.Name != original+".metadata" && !strings.HasPrefix(entry.Name, original+".") && !strings.HasPrefix(entry.Name, original+"/") {
			return nil, errors.New("unexpected file in snapshot")
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(reader, MaxDocumentBytes+1))
		reader.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > int(MaxDocumentBytes) {
			return nil, errors.New("snapshot entry too large")
		}
		if entry.Name == original+".metadata" {
			var metadata map[string]interface{}
			if err = json.Unmarshal(data, &metadata); err != nil {
				return nil, err
			}
			metadata["visibleName"] = name
			metadata["parent"] = parent
			metadata["lastModified"] = fmt.Sprint(time.Now().UnixMilli())
			metadata["deleted"] = false
			metadata["version"] = 1
			data, err = json.Marshal(metadata)
			if err != nil {
				return nil, err
			}
		}
		target := id + strings.TrimPrefix(entry.Name, original)
		if strings.Contains(target, "..") || strings.Contains(target, "\\") {
			return nil, errors.New("invalid archive path")
		}
		out, err := writer.Create(target)
		if err != nil {
			return nil, err
		}
		if _, err = out.Write(data); err != nil {
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	if _, err = output.Seek(0, 0); err != nil {
		return nil, err
	}
	success = true
	return output, nil
}
