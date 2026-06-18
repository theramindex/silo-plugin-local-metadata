package provider

import (
	"context"

	"github.com/theramindex/silo-local-metadata/internal/sidecar"
)

type MetadataRequest struct {
	ContentType string
	FilePath    string
}

type Provider struct {
	sidecars *sidecar.Provider
}

func NewProvider() *Provider {
	return &Provider{sidecars: sidecar.NewProvider()}
}

func NewProviderWithSidecars(sidecars *sidecar.Provider) *Provider {
	return &Provider{sidecars: sidecars}
}

func (p *Provider) GetMetadata(_ context.Context, req MetadataRequest) (*sidecar.LookupResult, error) {
	return p.sidecars.Lookup(req.FilePath)
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
