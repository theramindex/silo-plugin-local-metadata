package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	CapabilityID = "local-metadata"
	Scheme       = CapabilityID + "://"
)

type LookupResult struct {
	ProviderID string
	Item       Item
	Images     []Image
}

type Item struct {
	Title            string
	OriginalTitle    string
	SortTitle        string
	Overview         string
	Tagline          string
	Year             int
	RuntimeMinutes   int
	Genres           []string
	Studios          []string
	Countries        []string
	ContentRating    string
	OriginalLanguage string
	ReleaseDate      string
	AirDate          string
	Ratings          map[string]float64
	ProviderIDs      map[string]string
	Metadata         map[string]any
	People           []Person
}

type Person struct {
	Name      string
	Kind      string
	Character string
	SortOrder int
}

type Image struct {
	Kind string
	Path string
}

type Provider struct {
	fs FS
}

type FS interface {
	Open(name string) (io.ReadCloser, error)
	Stat(name string) (os.FileInfo, error)
}

type osFS struct{}

func (osFS) Open(name string) (io.ReadCloser, error) { return os.Open(name) }
func (osFS) Stat(name string) (os.FileInfo, error)   { return os.Stat(name) }

func NewProvider() *Provider {
	return &Provider{fs: osFS{}}
}

func NewProviderWithFS(fs FS) *Provider {
	return &Provider{fs: fs}
}

func (p *Provider) Lookup(mediaPath string) (*LookupResult, error) {
	mediaPath = strings.TrimSpace(mediaPath)
	if mediaPath == "" {
		return nil, nil
	}

	var item Item
	nfoPath := sidecarPath(mediaPath, ".nfo")
	if exists(p.fs, nfoPath) {
		parsed, err := p.parseNFO(nfoPath)
		if err != nil {
			return nil, fmt.Errorf("parse nfo %q: %w", nfoPath, err)
		}
		item = parsed
		item.Metadata = ensureMetadata(item.Metadata)
		item.Metadata["sidecar_nfo_path"] = nfoPath
	}

	images := p.findImages(mediaPath)
	if isZeroItem(item) && len(images) == 0 {
		return nil, nil
	}

	return &LookupResult{
		ProviderID: providerID(mediaPath),
		Item:       item,
		Images:     images,
	}, nil
}

func (p *Provider) ResolveImage(path string) (string, error) {
	localPath, ok := strings.CutPrefix(path, Scheme)
	if !ok {
		return "", nil
	}
	if !exists(p.fs, localPath) {
		return "", nil
	}
	abs, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}
	return "file://" + filepath.ToSlash(abs), nil
}

func (p *Provider) parseNFO(path string) (Item, error) {
	rc, err := p.fs.Open(path)
	if err != nil {
		return Item{}, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var item Item
	var currentPerson *Person
	var currentRatingName string
	var stack []string
	text := map[int]*strings.Builder{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Item{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			stack = append(stack, name)
			text[len(stack)] = &strings.Builder{}
			switch name {
			case "actor", "director", "writer":
				kind := name
				if kind == "actor" {
					kind = "cast"
				}
				currentPerson = &Person{Kind: kind, SortOrder: len(item.People)}
			case "rating":
				currentRatingName = attr(t, "name")
				if currentRatingName == "" {
					currentRatingName = "default"
				}
			}
		case xml.CharData:
			if len(stack) > 0 {
				text[len(stack)].Write([]byte(t))
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			depth := len(stack)
			value := ""
			if b := text[depth]; b != nil {
				value = strings.TrimSpace(b.String())
			}
			if value != "" {
				applyField(&item, currentPerson, currentRatingName, name, value)
			}
			switch name {
			case "actor", "director", "writer":
				if currentPerson != nil && currentPerson.Name != "" {
					item.People = append(item.People, *currentPerson)
				}
				currentPerson = nil
			case "rating":
				currentRatingName = ""
			}
			delete(text, depth)
			if depth > 0 {
				stack = stack[:depth-1]
			}
		}
	}

	return item, nil
}

func applyField(item *Item, person *Person, ratingName, name, value string) {
	if value == "" {
		return
	}
	if person != nil {
		switch name {
		case "name":
			person.Name = value
		case "role":
			person.Character = value
		case "order":
			if n, ok := parseInt(value); ok {
				person.SortOrder = n
			}
		}
		return
	}
	switch name {
	case "title":
		item.Title = value
	case "originaltitle":
		item.OriginalTitle = value
	case "sorttitle":
		item.SortTitle = value
	case "plot", "outline":
		if item.Overview == "" {
			item.Overview = value
		}
	case "tagline":
		item.Tagline = value
	case "year":
		item.Year, _ = parseInt(value)
	case "runtime":
		item.RuntimeMinutes = parseRuntimeMinutes(value)
	case "genre":
		item.Genres = appendUnique(item.Genres, splitList(value)...)
	case "studio":
		item.Studios = appendUnique(item.Studios, splitList(value)...)
	case "country":
		item.Countries = appendUnique(item.Countries, splitList(value)...)
	case "mpaa", "certification", "contentrating":
		item.ContentRating = value
	case "original_language", "originallanguage":
		item.OriginalLanguage = strings.ToLower(value)
	case "premiered", "releasedate":
		item.ReleaseDate = value
	case "aired":
		item.AirDate = value
	case "imdbid":
		item.ProviderIDs = ensureStringMap(item.ProviderIDs)
		item.ProviderIDs["imdb"] = value
	case "tmdbid":
		item.ProviderIDs = ensureStringMap(item.ProviderIDs)
		item.ProviderIDs["tmdb"] = value
	case "tvdbid":
		item.ProviderIDs = ensureStringMap(item.ProviderIDs)
		item.ProviderIDs["tvdb"] = value
	case "value":
		if ratingName != "" {
			if n, ok := parseFloat(value); ok {
				item.Ratings = ensureFloatMap(item.Ratings)
				item.Ratings[strings.ToLower(ratingName)] = n
			}
		}
	case "userrating", "rating":
		if n, ok := parseFloat(value); ok {
			item.Ratings = ensureFloatMap(item.Ratings)
			item.Ratings["default"] = n
		}
	case "season", "episode":
		if n, ok := parseInt(value); ok {
			item.Metadata = ensureMetadata(item.Metadata)
			item.Metadata[name+"_number"] = n
		}
	}
}

func (p *Provider) findImages(mediaPath string) []Image {
	var images []Image
	for _, candidate := range imageCandidates(mediaPath) {
		if exists(p.fs, candidate.path) {
			images = append(images, Image{Kind: candidate.kind, Path: candidate.path})
		}
	}
	return images
}

type imageCandidate struct {
	kind string
	path string
}

func imageCandidates(mediaPath string) []imageCandidate {
	base := trimExt(mediaPath)
	var out []imageCandidate
	for _, spec := range []struct {
		kind   string
		suffix []string
	}{
		{"poster", []string{"-poster", ".poster"}},
		{"backdrop", []string{"-backdrop", ".backdrop", "-fanart", ".fanart"}},
		{"logo", []string{"-logo", ".logo"}},
		{"still", []string{"-thumb", ".thumb", "-still", ".still"}},
	} {
		for _, suffix := range spec.suffix {
			for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
				out = append(out, imageCandidate{kind: spec.kind, path: base + suffix + ext})
			}
		}
	}
	return out
}

func sidecarPath(mediaPath, ext string) string {
	return trimExt(mediaPath) + ext
}

func trimExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func exists(fs FS, path string) bool {
	if path == "" {
		return false
	}
	info, err := fs.Stat(path)
	return err == nil && !info.IsDir()
}

func providerID(mediaPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(mediaPath)))
	return hex.EncodeToString(sum[:])[:24]
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]bool, len(dst)+len(values))
	for _, value := range dst {
		seen[strings.ToLower(value)] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if !seen[key] {
			dst = append(dst, value)
			seen[key] = true
		}
	}
	return dst
}

func splitList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '/' || r == '|'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 && strings.TrimSpace(value) != "" {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func parseRuntimeMinutes(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	if n, ok := parseInt(value); ok {
		return n
	}
	parts := strings.Fields(value)
	for i, part := range parts {
		if part == "min" || part == "mins" || part == "minutes" {
			if i > 0 {
				n, _ := parseInt(parts[i-1])
				return n
			}
		}
	}
	return 0
}

func parseInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	fields := strings.Fields(value)
	if len(fields) > 0 {
		value = fields[0]
	}
	n, err := strconv.Atoi(value)
	return n, err == nil
}

func parseFloat(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil
}

func attr(element xml.StartElement, name string) string {
	for _, a := range element.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

func ensureStringMap(value map[string]string) map[string]string {
	if value != nil {
		return value
	}
	return make(map[string]string)
}

func ensureFloatMap(value map[string]float64) map[string]float64 {
	if value != nil {
		return value
	}
	return make(map[string]float64)
}

func ensureMetadata(value map[string]any) map[string]any {
	if value != nil {
		return value
	}
	return make(map[string]any)
}

func isZeroItem(item Item) bool {
	return item.Title == "" &&
		item.OriginalTitle == "" &&
		item.SortTitle == "" &&
		item.Overview == "" &&
		item.Tagline == "" &&
		item.Year == 0 &&
		item.RuntimeMinutes == 0 &&
		len(item.Genres) == 0 &&
		len(item.Studios) == 0 &&
		len(item.Countries) == 0 &&
		item.ContentRating == "" &&
		item.OriginalLanguage == "" &&
		item.ReleaseDate == "" &&
		item.AirDate == "" &&
		len(item.Ratings) == 0 &&
		len(item.ProviderIDs) == 0 &&
		len(item.Metadata) == 0 &&
		len(item.People) == 0
}
