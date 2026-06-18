package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-local-metadata/provider"
)

func TestMetadataServerGetMetadataUsesFilePathSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	media := filepath.Join(dir, "Movie.mkv")
	mustWrite(t, media, "")
	mustWrite(t, filepath.Join(dir, "Movie.nfo"), `<movie><title>Local Movie</title><year>2025</year></movie>`)
	mustWrite(t, filepath.Join(dir, "Movie-poster.jpg"), "jpg")

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.GetMetadata(context.Background(), &pluginv1.GetMetadataRequest{
		ItemType: "movie",
		FilePath: media,
	})
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if resp.GetItem().GetTitle() != "Local Movie" {
		t.Fatalf("Title = %q", resp.GetItem().GetTitle())
	}
	if resp.GetItem().GetYear() != 2025 {
		t.Fatalf("Year = %d", resp.GetItem().GetYear())
	}
	if resp.GetItem().GetPosterPath() == "" {
		t.Fatal("PosterPath is empty")
	}
	if got := resp.GetItem().GetProviderIds().AsMap()["local-metadata"]; got == "" {
		t.Fatalf("local-metadata provider id missing from ProviderIds: %#v", resp.GetItem().GetProviderIds().AsMap())
	}
}

func TestMetadataServerGetMetadataNoSidecarReturnsEmptyResponse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	media := filepath.Join(dir, "Movie.mkv")
	mustWrite(t, media, "")

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.GetMetadata(context.Background(), &pluginv1.GetMetadataRequest{
		ItemType: "movie",
		FilePath: media,
	})
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if resp == nil {
		t.Fatal("GetMetadata() returned nil response")
	}
	if resp.GetItem() != nil {
		t.Fatalf("GetMetadata().Item = %#v, want nil", resp.GetItem())
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
