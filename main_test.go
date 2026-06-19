package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-local-metadata/provider"
)

func TestMetadataServerSearchReturnsSyntheticLocalCandidate(t *testing.T) {
	t.Parallel()

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.Search(context.Background(), &pluginv1.SearchMetadataRequest{
		Query:    "Local Movie",
		ItemType: "movie",
		Year:     2025,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	results := resp.GetResults()
	if len(results) != 1 {
		t.Fatalf("Search() results length = %d, want 1", len(results))
	}
	result := results[0]
	if result.GetTitle() != "Local Movie" {
		t.Fatalf("Title = %q", result.GetTitle())
	}
	if result.GetProviderId() == "" {
		t.Fatal("ProviderId is empty")
	}
	providerIDs := result.GetProviderIds().AsMap()
	if got := providerIDs["local"]; got != result.GetProviderId() {
		t.Fatalf("provider_ids[local] = %v, want %q", got, result.GetProviderId())
	}
	if got := providerIDs["local-metadata"]; got != result.GetProviderId() {
		t.Fatalf("provider_ids[local-metadata] = %v, want %q", got, result.GetProviderId())
	}
}

func TestMetadataServerSearchReturnsSyntheticCandidateForEmptyQuery(t *testing.T) {
	t.Parallel()

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.Search(context.Background(), &pluginv1.SearchMetadataRequest{
		ItemType: "movie",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	results := resp.GetResults()
	if len(results) != 1 {
		t.Fatalf("Search() results length = %d, want 1", len(results))
	}
	if got := results[0].GetTitle(); got != "Local Metadata" {
		t.Fatalf("Title = %q, want Local Metadata", got)
	}
	if got := results[0].GetYear(); got <= 0 {
		t.Fatalf("Year = %d, want positive fallback year", got)
	}
}

func TestMetadataServerSearchInfersPersianCalendarYear(t *testing.T) {
	t.Parallel()

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.Search(context.Background(), &pluginv1.SearchMetadataRequest{
		Query:    "فیلم جدید درام عاشق پیشه (محصول سال 1402)",
		ItemType: "movie",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	results := resp.GetResults()
	if len(results) != 1 {
		t.Fatalf("Search() results length = %d, want 1", len(results))
	}
	if got := results[0].GetYear(); got != 2023 {
		t.Fatalf("Year = %d, want 2023", got)
	}
}

func TestMetadataServerSearchSkipsUnsupportedItemType(t *testing.T) {
	t.Parallel()

	ms := &metadataServer{
		runtime: &runtimeServer{provider: provider.NewProvider()},
	}
	resp, err := ms.Search(context.Background(), &pluginv1.SearchMetadataRequest{
		Query:    "Local Movie",
		ItemType: "album",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := len(resp.GetResults()); got != 0 {
		t.Fatalf("Search() results length = %d, want 0", got)
	}
}

func TestMetadataServerGetMetadataUsesFilePathSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	media := filepath.Join(dir, "Movie.mkv")
	mustWrite(t, media, "")
	mustWrite(t, filepath.Join(dir, "Movie.nfo"), `<movie>
  <title>Local Movie</title>
  <year>2025</year>
  <director>Local Director</director>
  <actor><name>Local Actor</name><role>Lead</role><order>1</order></actor>
</movie>`)
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
	if got := resp.GetItem().GetProviderIds().AsMap()["local"]; got == "" {
		t.Fatalf("local provider id missing from ProviderIds: %#v", resp.GetItem().GetProviderIds().AsMap())
	}
	people := resp.GetItem().GetPeople()
	if len(people) != 2 {
		t.Fatalf("People length = %d, people = %#v", len(people), people)
	}
	byName := map[string]*pluginv1.PersonRecord{}
	for _, person := range people {
		byName[person.GetName()] = person
	}
	if got := byName["Local Actor"].GetKind(); got != "Actor" {
		t.Fatalf("Local Actor Kind = %q, want Actor", got)
	}
	if got := byName["Local Actor"].GetCharacter(); got != "Lead" {
		t.Fatalf("Local Actor Character = %q, want Lead", got)
	}
	if got := byName["Local Director"].GetKind(); got != "Director" {
		t.Fatalf("Local Director Kind = %q, want Director", got)
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
