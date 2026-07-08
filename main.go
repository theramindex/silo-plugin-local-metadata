package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"github.com/theramindex/silo-plugin-local-metadata/internal/sidecar"
	"github.com/theramindex/silo-plugin-local-metadata/provider"
)

// version is set at build time via -ldflags "-X main.version=...".
var version string

const localProviderIDKey = "local"

var (
	gregorianYearPattern = regexp.MustCompile(`\b(19[0-9]{2}|20[0-9]{2})\b`)
	persianYearPattern   = regexp.MustCompile(`\b(13[0-9]{2}|14[0-9]{2})\b`)
)

type runtimeServer struct {
	pluginv1.UnimplementedRuntimeServer

	manifest *pluginv1.PluginManifest
	provider *provider.Provider
}

type metadataServer struct {
	pluginv1.UnimplementedMetadataProviderServer
	runtime *runtimeServer
}

//go:embed manifest.json
var manifestJSON []byte

func (s *runtimeServer) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

func (s *runtimeServer) Configure(context.Context, *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	return &pluginv1.ConfigureResponse{}, nil
}

func (s *metadataServer) Search(_ context.Context, req *pluginv1.SearchMetadataRequest) (*pluginv1.SearchMetadataResponse, error) {
	title := strings.TrimSpace(req.GetQuery())
	itemType := strings.TrimSpace(req.GetItemType())
	if !supportsSearchItemType(itemType) {
		debugf("local-metadata: Search skipped item_type=%q query=%q year=%d reason=unsupported_item_type", req.GetItemType(), req.GetQuery(), req.GetYear())
		return &pluginv1.SearchMetadataResponse{}, nil
	}
	indexed, err := s.runtime.provider.Search(context.Background(), provider.SearchRequest{
		ContentType: itemType,
		Query:       title,
		Year:        int(req.GetYear()),
		ProviderIDs: stringMapFromStruct(req.GetProviderIds()),
	})
	if err != nil {
		return nil, err
	}
	if indexed.Authoritative || indexed.IndexConfigured {
		results := make([]*pluginv1.ProviderSearchResult, 0, len(indexed.Results))
		for _, result := range indexed.Results {
			searchResult, err := providerSearchResultFromLookup(result, itemType)
			if err != nil {
				return nil, err
			}
			debugf("local-metadata: Search indexed matched item_type=%q query=%q year=%d provider_id=%q", itemType, title, searchResult.GetYear(), searchResult.GetProviderId())
			results = append(results, searchResult)
		}
		return &pluginv1.SearchMetadataResponse{Results: results}, nil
	}
	if title == "" {
		title = "Local Metadata"
	}
	year := localSearchYear(title, req.GetYear())

	providerID := localSearchProviderID(itemType, title, year)
	debugf("local-metadata: Search matched item_type=%q query=%q year=%d provider_id=%q", itemType, title, year, providerID)
	providerIDs, err := stringStruct(map[string]string{
		localProviderIDKey:   providerID,
		sidecar.CapabilityID: providerID,
	})
	if err != nil {
		return nil, err
	}
	return &pluginv1.SearchMetadataResponse{
		Results: []*pluginv1.ProviderSearchResult{
			{
				ProviderId:  providerID,
				ItemType:    itemType,
				Title:       title,
				Year:        year,
				ProviderIds: providerIDs,
			},
		},
	}, nil
}

func (s *metadataServer) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	result, err := s.runtime.provider.GetMetadata(ctx, provider.MetadataRequest{
		ContentType: req.GetItemType(),
		FilePath:    req.GetFilePath(),
		ProviderID:  req.GetProviderId(),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &pluginv1.GetMetadataResponse{}, nil
	}
	item, err := metadataItemFromResult(result, req.GetItemType())
	if err != nil {
		return nil, err
	}
	return &pluginv1.GetMetadataResponse{Item: item}, nil
}

func providerSearchResultFromLookup(result *sidecar.LookupResult, itemType string) (*pluginv1.ProviderSearchResult, error) {
	rawProviderIDs := map[string]string{
		localProviderIDKey:   result.ProviderID,
		sidecar.CapabilityID: result.ProviderID,
	}
	for key, value := range result.Item.ProviderIDs {
		if value != "" {
			rawProviderIDs[key] = value
		}
	}
	providerIDs, err := stringStruct(rawProviderIDs)
	if err != nil {
		return nil, err
	}
	return &pluginv1.ProviderSearchResult{
		ProviderId:    result.ProviderID,
		ItemType:      itemType,
		Title:         result.Item.Title,
		OriginalTitle: result.Item.OriginalTitle,
		Year:          int32(result.Item.Year),
		Overview:      result.Item.Overview,
		ImageUrl:      searchImageURL(result.Images),
		ProviderIds:   providerIDs,
	}, nil
}

func (s *metadataServer) GetPersonDetail(context.Context, *pluginv1.GetPersonDetailRequest) (*pluginv1.GetPersonDetailResponse, error) {
	return &pluginv1.GetPersonDetailResponse{}, nil
}

func (s *metadataServer) GetSeasons(context.Context, *pluginv1.GetSeasonsRequest) (*pluginv1.GetSeasonsResponse, error) {
	return &pluginv1.GetSeasonsResponse{}, nil
}

func (s *metadataServer) GetEpisodes(context.Context, *pluginv1.GetEpisodesRequest) (*pluginv1.GetEpisodesResponse, error) {
	return &pluginv1.GetEpisodesResponse{}, nil
}

func (s *metadataServer) GetImages(context.Context, *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	// SDK v0.7.0 does not pass file_path to GetImages. Local sidecar images are
	// still returned through GetMetadata's poster/backdrop/logo fields when Silo
	// provides file_path there.
	return &pluginv1.GetImagesResponse{}, nil
}

func (s *metadataServer) ResolveImageURL(ctx context.Context, req *pluginv1.ResolveImageURLRequest) (*pluginv1.ResolveImageURLResponse, error) {
	url, err := s.runtime.provider.ResolveImage(ctx, req.GetPath())
	if err != nil {
		return nil, err
	}
	return &pluginv1.ResolveImageURLResponse{Url: url}, nil
}

func (s *metadataServer) ResolveImageURLs(ctx context.Context, req *pluginv1.ResolveImageURLsRequest) (*pluginv1.ResolveImageURLsResponse, error) {
	urls := make(map[string]string, len(req.GetPaths()))
	for _, path := range req.GetPaths() {
		url, err := s.runtime.provider.ResolveImage(ctx, path)
		if err != nil {
			return nil, err
		}
		urls[path] = url
	}
	return &pluginv1.ResolveImageURLsResponse{Urls: urls}, nil
}

func main() {
	manifest, err := loadManifest()
	if err != nil {
		panic(err)
	}

	rs := &runtimeServer{
		manifest: manifest,
		provider: provider.NewProvider(),
	}

	runtime.Serve(runtime.ServeConfig{
		Servers: runtime.CapabilityServers{
			Runtime:          rs,
			MetadataProvider: &metadataServer{runtime: rs},
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}
	if version != "" {
		manifest.Version = version
	}

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	return manifest, nil
}

func metadataItemFromResult(result *sidecar.LookupResult, itemType string) (*pluginv1.MetadataItem, error) {
	rawProviderIDs := map[string]string{
		localProviderIDKey:   result.ProviderID,
		sidecar.CapabilityID: result.ProviderID,
	}
	for key, value := range result.Item.ProviderIDs {
		if value != "" {
			rawProviderIDs[key] = value
		}
	}
	providerIDs, err := stringStruct(rawProviderIDs)
	if err != nil {
		return nil, err
	}
	ratings, err := floatStruct(result.Item.Ratings)
	if err != nil {
		return nil, err
	}
	metadata, err := structpb.NewStruct(result.Item.Metadata)
	if err != nil && len(result.Item.Metadata) > 0 {
		return nil, err
	}

	item := &pluginv1.MetadataItem{
		ProviderId:        result.ProviderID,
		ItemType:          itemType,
		Title:             result.Item.Title,
		OriginalTitle:     result.Item.OriginalTitle,
		SortTitle:         result.Item.SortTitle,
		Year:              int32(result.Item.Year),
		Overview:          result.Item.Overview,
		Tagline:           result.Item.Tagline,
		Runtime:           int32(result.Item.RuntimeMinutes),
		Genres:            append([]string(nil), result.Item.Genres...),
		Studios:           append([]string(nil), result.Item.Studios...),
		Countries:         append([]string(nil), result.Item.Countries...),
		OriginalLanguage:  result.Item.OriginalLanguage,
		ContentRating:     result.Item.ContentRating,
		ProviderIds:       providerIDs,
		Ratings:           ratings,
		Metadata:          metadata,
		ReleaseDate:       result.Item.ReleaseDate,
		People:            peopleToRecords(result.Item.People),
		BackdropThumbhash: "",
		PosterThumbhash:   "",
	}
	for _, image := range result.Images {
		switch image.Kind {
		case "poster":
			if item.PosterPath == "" {
				item.PosterPath = sidecar.Scheme + image.Path
			}
		case "backdrop":
			if item.BackdropPath == "" {
				item.BackdropPath = sidecar.Scheme + image.Path
			}
		case "logo":
			if item.LogoPath == "" {
				item.LogoPath = sidecar.Scheme + image.Path
			}
		}
	}
	if item.ReleaseDate == "" {
		item.ReleaseDate = result.Item.AirDate
	}
	return item, nil
}

func peopleToRecords(people []sidecar.Person) []*pluginv1.PersonRecord {
	if len(people) == 0 {
		return nil
	}
	records := make([]*pluginv1.PersonRecord, 0, len(people))
	for _, person := range people {
		records = append(records, &pluginv1.PersonRecord{
			Name:      person.Name,
			Kind:      person.Kind,
			Character: person.Character,
			SortOrder: int32(person.SortOrder),
		})
	}
	return records
}

func stringStruct(value map[string]string) (*structpb.Struct, error) {
	if len(value) == 0 {
		return nil, nil
	}
	converted := make(map[string]any, len(value))
	for key, entry := range value {
		if entry != "" {
			converted[key] = entry
		}
	}
	if len(converted) == 0 {
		return nil, nil
	}
	return structpb.NewStruct(converted)
}

func stringMapFromStruct(value *structpb.Struct) map[string]string {
	if value == nil {
		return nil
	}
	out := make(map[string]string, len(value.GetFields()))
	for key, entry := range value.GetFields() {
		if stringValue := strings.TrimSpace(entry.GetStringValue()); stringValue != "" {
			out[key] = stringValue
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func floatStruct(value map[string]float64) (*structpb.Struct, error) {
	if len(value) == 0 {
		return nil, nil
	}
	converted := make(map[string]any, len(value))
	for key, entry := range value {
		converted[key] = entry
	}
	return structpb.NewStruct(converted)
}

func searchImageURL(images []sidecar.Image) string {
	for _, image := range images {
		if image.Kind == "poster" && image.Path != "" {
			return sidecar.Scheme + image.Path
		}
	}
	for _, image := range images {
		if image.Path != "" {
			return sidecar.Scheme + image.Path
		}
	}
	return ""
}

func supportsSearchItemType(itemType string) bool {
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "movie", "musicvideo", "music_video", "series", "show", "tvshow", "tv_show", "season", "episode":
		return true
	default:
		return false
	}
}

func localSearchProviderID(itemType, title string, year int32) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("local-search\x00%s\x00%s\x00%d", strings.ToLower(itemType), strings.ToLower(title), year)))
	return hex.EncodeToString(sum[:])[:24]
}

func localSearchYear(title string, requestYear int32) int32 {
	if requestYear > 0 {
		return requestYear
	}
	if year := parseSearchYear(gregorianYearPattern, title); year > 0 {
		return year
	}
	if year := parseSearchYear(persianYearPattern, title); year > 0 {
		return year + 621
	}
	return int32(time.Now().Year())
}

func parseSearchYear(pattern *regexp.Regexp, title string) int32 {
	match := pattern.FindString(title)
	if match == "" {
		return 0
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return int32(year)
}

func debugf(format string, args ...any) {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("SILO_LOCAL_METADATA_DEBUG")))
	if value != "1" && value != "true" && value != "yes" && value != "on" {
		return
	}
	path := strings.TrimSpace(os.Getenv("SILO_LOCAL_METADATA_DEBUG_LOG"))
	if path == "" {
		path = "/tmp/silo-local-metadata-debug.log"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, format+"\n", args...)
}
