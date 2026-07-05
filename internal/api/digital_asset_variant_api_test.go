package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
)

type digitalVariantTestNode struct {
	mockNode
	assets contracts.DigitalAssetService
}

func (n *digitalVariantTestNode) DigitalAssets() contracts.DigitalAssetService {
	return n.assets
}

func TestDigitalAssetLicenseKeyAPIHandlesVariantPools(t *testing.T) {
	assets := newRecordingDigitalAssetService()
	ts := digitalAssetTestServer(t, &digitalVariantTestNode{assets: assets})

	importBody := []byte(`{
		"listingSlug": "listing-variant",
		"variantSku": "sku-blue",
		"appId": "app-blue",
		"keys": ["BLUE-1", "BLUE-2"],
		"licenseType": "perpetual",
		"maxActivations": 1
	}`)
	resp, body := digitalAssetDoReq(t, ts, http.MethodPost, "/v1/digital-assets/license-keys", importBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import status = %d body=%s", resp.StatusCode, string(body))
	}
	var importEnvelope struct {
		Data struct {
			Imported int `json:"imported"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &importEnvelope); err != nil {
		t.Fatalf("unmarshal import response: %v", err)
	}
	if importEnvelope.Data.Imported != 2 {
		t.Fatalf("imported = %d, want 2", importEnvelope.Data.Imported)
	}
	if got := assets.imports[0].variantSKU; got != "sku-blue" {
		t.Fatalf("variant passed to import = %q, want sku-blue", got)
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets/license-keys/stats?listingSlug=listing-variant&variantSku=sku-blue", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d body=%s", resp.StatusCode, string(body))
	}
	var statsEnvelope struct {
		Data contracts.LicenseKeyPoolStats `json:"data"`
	}
	if err := json.Unmarshal(body, &statsEnvelope); err != nil {
		t.Fatalf("unmarshal stats response: %v", err)
	}
	if statsEnvelope.Data.Available != 2 || statsEnvelope.Data.Total != 2 {
		t.Fatalf("stats = %+v, want available=2 total=2", statsEnvelope.Data)
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets/license-keys?listingSlug=listing-variant&variantSku=sku-blue", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d body=%s", resp.StatusCode, string(body))
	}
	var listEnvelope struct {
		Data []contracts.MaskedLicenseKey `json:"data"`
	}
	if err := json.Unmarshal(body, &listEnvelope); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listEnvelope.Data) != 2 {
		t.Fatalf("listed keys = %d, want 2", len(listEnvelope.Data))
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets/license-keys/stats?listingSlug=listing-variant", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("universal stats status = %d body=%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &statsEnvelope); err != nil {
		t.Fatalf("unmarshal universal stats response: %v", err)
	}
	if statsEnvelope.Data.Total != 0 {
		t.Fatalf("universal pool total = %d, want 0", statsEnvelope.Data.Total)
	}
}

func TestDigitalAssetLinkAPIHandlesVariantAssets(t *testing.T) {
	assets := newRecordingDigitalAssetService()
	ts := digitalAssetTestServer(t, &digitalVariantTestNode{assets: assets})

	createBody := []byte(`{
		"listingSlug": "listing-link",
		"variantSku": "sku-blue",
		"url": "https://example.com/blue-download"
	}`)
	resp, body := digitalAssetDoReq(t, ts, http.MethodPost, "/v1/digital-assets/link", createBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create link status = %d body=%s", resp.StatusCode, string(body))
	}
	var createEnvelope struct {
		Data contracts.DigitalAssetInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &createEnvelope); err != nil {
		t.Fatalf("unmarshal create link response: %v", err)
	}
	if createEnvelope.Data.VariantSKU != "sku-blue" {
		t.Fatalf("response variantSku = %q, want sku-blue", createEnvelope.Data.VariantSKU)
	}
	if len(assets.links) != 1 || assets.links[0].variantSKU != "sku-blue" {
		t.Fatalf("recorded links = %+v, want one sku-blue link", assets.links)
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets?listingSlug=listing-link&variantSku=sku-blue", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("variant list status = %d body=%s", resp.StatusCode, string(body))
	}
	var listEnvelope struct {
		Data []contracts.DigitalAssetInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &listEnvelope); err != nil {
		t.Fatalf("unmarshal variant list response: %v", err)
	}
	if len(listEnvelope.Data) != 1 {
		t.Fatalf("variant asset count = %d, want 1", len(listEnvelope.Data))
	}
	if listEnvelope.Data[0].VariantSKU != "sku-blue" || listEnvelope.Data[0].URL != "https://example.com/blue-download" {
		t.Fatalf("variant asset = %+v, want sku-blue link with URL", listEnvelope.Data[0])
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets?listingSlug=listing-link", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("universal list status = %d body=%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &listEnvelope); err != nil {
		t.Fatalf("unmarshal universal list response: %v", err)
	}
	if len(listEnvelope.Data) != 0 {
		t.Fatalf("universal asset count = %d, want 0", len(listEnvelope.Data))
	}
}

func TestDigitalAssetUploadStreamAPIHandlesVariantFiles(t *testing.T) {
	assets := newRecordingDigitalAssetService()
	ts := digitalAssetTestServer(t, &digitalVariantTestNode{assets: assets})

	resp, body := digitalAssetDoMultipartReq(t, ts, "/v1/digital-assets/upload-stream", map[string]string{
		"listingSlug": "listing-file",
		"variantSku":  "sku-blue",
		"mimeType":    "application/pdf",
	}, "file", "blue-guide.pdf", "application/pdf", []byte("blue pdf payload"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d body=%s", resp.StatusCode, string(body))
	}
	var uploadEnvelope struct {
		Data contracts.DigitalAssetInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &uploadEnvelope); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	if uploadEnvelope.Data.VariantSKU != "sku-blue" {
		t.Fatalf("response variantSku = %q, want sku-blue", uploadEnvelope.Data.VariantSKU)
	}
	if uploadEnvelope.Data.FileName != "blue-guide.pdf" || uploadEnvelope.Data.MimeType != "application/pdf" {
		t.Fatalf("response file metadata = %+v, want blue-guide.pdf application/pdf", uploadEnvelope.Data)
	}
	if len(assets.uploads) != 1 {
		t.Fatalf("recorded uploads = %d, want 1", len(assets.uploads))
	}
	if assets.uploads[0].variantSKU != "sku-blue" || string(assets.uploads[0].data) != "blue pdf payload" {
		t.Fatalf("recorded upload = %+v, want sku-blue payload", assets.uploads[0])
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets?listingSlug=listing-file&variantSku=sku-blue", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("variant list status = %d body=%s", resp.StatusCode, string(body))
	}
	var listEnvelope struct {
		Data []contracts.DigitalAssetInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &listEnvelope); err != nil {
		t.Fatalf("unmarshal variant list response: %v", err)
	}
	if len(listEnvelope.Data) != 1 || listEnvelope.Data[0].VariantSKU != "sku-blue" {
		t.Fatalf("variant file list = %+v, want one sku-blue file", listEnvelope.Data)
	}

	resp, body = digitalAssetDoReq(t, ts, http.MethodGet, "/v1/digital-assets?listingSlug=listing-file", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("universal list status = %d body=%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &listEnvelope); err != nil {
		t.Fatalf("unmarshal universal list response: %v", err)
	}
	if len(listEnvelope.Data) != 0 {
		t.Fatalf("universal file list count = %d, want 0", len(listEnvelope.Data))
	}
}

func digitalAssetTestServer(t *testing.T, node contracts.NodeService) *httptest.Server {
	t.Helper()
	gateway := &Gateway{config: &GatewayConfig{}}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), nodeContextKey, node)
			ctx = WithAuthIdentity(ctx, &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", mustNewV1Router(t, gateway, false, false))
	ts := httptest.NewServer(outer)
	t.Cleanup(ts.Close)
	return ts
}

func digitalAssetDoReq(t *testing.T, ts *httptest.Server, method, path string, body []byte) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, data
}

func digitalAssetDoMultipartReq(
	t *testing.T,
	ts *httptest.Server,
	path string,
	fields map[string]string,
	fileField string,
	fileName string,
	contentType string,
	fileData []byte,
) (*http.Response, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, key := range []string{"listingSlug", "variantSku", "mimeType"} {
		if value, ok := fields[key]; ok {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileField, fileName))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(fileData); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+path, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, data
}

type recordingDigitalAssetService struct {
	imports []recordedLicenseKeyImport
	links   []recordedLinkAsset
	uploads []recordedFileUpload
	keys    map[string][]contracts.MaskedLicenseKey
	assets  map[string][]contracts.DigitalAssetInfo
}

type recordedLicenseKeyImport struct {
	listingSlug string
	variantSKU  string
	appID       string
}

type recordedLinkAsset struct {
	listingSlug string
	variantSKU  string
	url         string
}

type recordedFileUpload struct {
	listingSlug string
	variantSKU  string
	fileName    string
	mimeType    string
	data        []byte
}

func newRecordingDigitalAssetService() *recordingDigitalAssetService {
	return &recordingDigitalAssetService{
		keys:   make(map[string][]contracts.MaskedLicenseKey),
		assets: make(map[string][]contracts.DigitalAssetInfo),
	}
}

func digitalPoolKey(listingSlug string, variantSKU string) string {
	return listingSlug + "\x00" + variantSKU
}

func (s *recordingDigitalAssetService) ImportLicenseKeys(listingSlug, variantSKU, appID string, keys []string, licenseType string, maxActivations int, expiresAt time.Time) (int, error) {
	s.imports = append(s.imports, recordedLicenseKeyImport{listingSlug: listingSlug, variantSKU: variantSKU, appID: appID})
	poolKey := digitalPoolKey(listingSlug, variantSKU)
	for _, key := range keys {
		s.keys[poolKey] = append(s.keys[poolKey], contracts.MaskedLicenseKey{
			ID:             fmt.Sprintf("%s-%d", variantSKU, len(s.keys[poolKey])+1),
			Status:         "available",
			MaskedKey:      maskTestLicenseKey(key),
			LicenseType:    licenseType,
			MaxActivations: maxActivations,
		})
	}
	return len(keys), nil
}

func (s *recordingDigitalAssetService) GetLicenseKeyPoolStats(listingSlug, variantSKU string) (*contracts.LicenseKeyPoolStats, error) {
	keys := s.keys[digitalPoolKey(listingSlug, variantSKU)]
	var stats contracts.LicenseKeyPoolStats
	stats.Total = int64(len(keys))
	for _, key := range keys {
		switch key.Status {
		case "available":
			stats.Available++
		case "dispensed":
			stats.Dispensed++
		case "revoked":
			stats.Revoked++
		}
	}
	return &stats, nil
}

func (s *recordingDigitalAssetService) ListLicenseKeys(listingSlug, variantSKU string, limit, offset int) ([]contracts.MaskedLicenseKey, error) {
	keys := s.keys[digitalPoolKey(listingSlug, variantSKU)]
	if offset >= len(keys) {
		return nil, nil
	}
	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}
	return append([]contracts.MaskedLicenseKey(nil), keys[offset:end]...), nil
}

func maskTestLicenseKey(key string) string {
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return key[:2] + strings.Repeat("*", len(key)-4) + key[len(key)-2:]
}

func (s *recordingDigitalAssetService) GetBuyerDigitalAssets(string, string, string, bool, int64) ([]contracts.BuyerAssetEntry, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) GetDigitalDeliveryStatus(string, string, string, bool) (*contracts.DigitalDeliveryStatus, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) RetryDigitalDelivery(string, string, bool) (*contracts.DigitalDeliveryStatus, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) ServeDownload(context.Context, contracts.DownloadRequest) (*contracts.DownloadResponse, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) ValidateLicense(string, string) (*contracts.LicenseValidationResult, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) ActivateLicense(string, string, string, string, string) (*contracts.LicenseActivationResult, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) DeactivateLicense(string, string, string) error {
	return nil
}
func (s *recordingDigitalAssetService) UploadFileAssetStream(_ context.Context, listingSlug, variantSKU, fileName, mimeType string, src io.Reader, _ int64) (*contracts.DigitalAssetInfo, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	s.uploads = append(s.uploads, recordedFileUpload{
		listingSlug: listingSlug,
		variantSKU:  variantSKU,
		fileName:    fileName,
		mimeType:    mimeType,
		data:        data,
	})
	info := contracts.DigitalAssetInfo{
		ID:          fmt.Sprintf("file-%d", len(s.uploads)),
		ListingSlug: listingSlug,
		VariantSKU:  variantSKU,
		AssetType:   "file",
		FileName:    fileName,
		FileSize:    int64(len(data)),
		MimeType:    mimeType,
	}
	poolKey := digitalPoolKey(listingSlug, variantSKU)
	s.assets[poolKey] = append(s.assets[poolKey], info)
	return &info, nil
}
func (s *recordingDigitalAssetService) CreateLinkAsset(listingSlug, variantSKU, url string) (*contracts.DigitalAssetInfo, error) {
	s.links = append(s.links, recordedLinkAsset{listingSlug: listingSlug, variantSKU: variantSKU, url: url})
	info := contracts.DigitalAssetInfo{
		ID:          fmt.Sprintf("link-%d", len(s.links)),
		ListingSlug: listingSlug,
		VariantSKU:  variantSKU,
		AssetType:   "link",
		URL:         url,
	}
	poolKey := digitalPoolKey(listingSlug, variantSKU)
	s.assets[poolKey] = append(s.assets[poolKey], info)
	return &info, nil
}
func (s *recordingDigitalAssetService) CreateLicenseKeyAsset(string, string, string) (*contracts.DigitalAssetInfo, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) GetAssetsByListing(listingSlug, variantSKU string) ([]contracts.DigitalAssetInfo, error) {
	assets := s.assets[digitalPoolKey(listingSlug, variantSKU)]
	return append([]contracts.DigitalAssetInfo(nil), assets...), nil
}
func (s *recordingDigitalAssetService) GetAssetByID(string) (*contracts.DigitalAssetInfo, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) UpdateAsset(string, contracts.AssetUpdateInput) (*contracts.DigitalAssetInfo, error) {
	return nil, nil
}
func (s *recordingDigitalAssetService) DeleteAsset(string) error {
	return nil
}
func (s *recordingDigitalAssetService) RevokeLicenseKey(string) error {
	return nil
}

var _ contracts.DigitalAssetService = (*recordingDigitalAssetService)(nil)
