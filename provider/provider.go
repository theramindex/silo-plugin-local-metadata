package provider

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/theramindex/silo-plugin-local-metadata/internal/sidecar"
)

type MetadataRequest struct {
	ContentType string
	FilePath    string
}

type Provider struct {
	sidecars *sidecar.Provider
	debug    bool
	debugLog string
}

func NewProvider() *Provider {
	return &Provider{
		sidecars: sidecar.NewProvider(),
		debug:    debugEnabled(),
		debugLog: debugLogPath(),
	}
}

func NewProviderWithSidecars(sidecars *sidecar.Provider) *Provider {
	return &Provider{
		sidecars: sidecars,
		debug:    debugEnabled(),
		debugLog: debugLogPath(),
	}
}

func (p *Provider) GetMetadata(_ context.Context, req MetadataRequest) (*sidecar.LookupResult, error) {
	if strings.TrimSpace(req.FilePath) == "" {
		p.debugf("local-metadata: GetMetadata missing file_path item_type=%q", req.ContentType)
	}
	if p.debug {
		diag := p.sidecars.Diagnostics(req.FilePath, req.ContentType)
		p.debugf(
			"local-metadata: GetMetadata request item_type=%q file_path=%q nfo_found=%q nfo_candidates=%q image_count=%d",
			req.ContentType,
			diag.MediaPath,
			diag.NFOPath,
			strings.Join(diag.NFOCandidates, "|"),
			diag.ImageCount,
		)
	}

	result, err := p.sidecars.Lookup(req.FilePath, req.ContentType)
	if p.debug {
		switch {
		case err != nil:
			p.debugf("local-metadata: GetMetadata error item_type=%q file_path=%q error=%v", req.ContentType, strings.TrimSpace(req.FilePath), err)
		case result == nil:
			p.debugf("local-metadata: GetMetadata empty item_type=%q file_path=%q", req.ContentType, strings.TrimSpace(req.FilePath))
		default:
			p.debugf(
				"local-metadata: GetMetadata matched item_type=%q file_path=%q provider_id=%q title=%q year=%d image_count=%d",
				req.ContentType,
				strings.TrimSpace(req.FilePath),
				result.ProviderID,
				result.Item.Title,
				result.Item.Year,
				len(result.Images),
			)
		}
	}
	return result, err
}

func (p *Provider) GetImages(_ context.Context, filePath string) ([]sidecar.Image, error) {
	result, err := p.sidecars.Lookup(filePath)
	if err != nil || result == nil {
		return nil, err
	}
	return result.Images, nil
}

func (p *Provider) ResolveImage(_ context.Context, path string) (string, error) {
	return p.sidecars.ResolveImage(path)
}

func debugEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("SILO_LOCAL_METADATA_DEBUG")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func debugLogPath() string {
	if value := strings.TrimSpace(os.Getenv("SILO_LOCAL_METADATA_DEBUG_LOG")); value != "" {
		return value
	}
	return "/tmp/silo-local-metadata-debug.log"
}

func (p *Provider) debugf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	log.Print(message)
	if !p.debug && !strings.Contains(message, "missing file_path") {
		return
	}
	if p.debugLog == "" {
		return
	}
	file, err := os.OpenFile(p.debugLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("local-metadata: open debug log %q: %v", p.debugLog, err)
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintln(file, message)
}
