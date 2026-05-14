//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/h2non/filetype"

	"github.com/mobazha/mobazha3.0/pkg/response"
)

// DG-1.9 — Gumroad Import Tool
//
// MVP scope (per DIGITAL_DELIVERY_DESIGN.md §1.7):
//   - Read-only Gumroad API integration via personal access token (no OAuth
//     flow for MVP — adding OAuth requires app registration + redirect domain
//     config that we'd rather defer until after we see real adoption).
//   - Import metadata (title/description/price/tags) and the public thumbnail.
//   - Land all imports as DRAFTS so the seller reviews each one before
//     publishing. Gumroad descriptions are HTML-formatted and may need
//     cleanup, prices need a sanity check, and the seller still has to attach
//     the actual digital file before publishing.
//   - Skip products that require shipping (PHYSICAL_GOOD path needs a shipping
//     profile picker we haven't designed yet).
//   - Surface a clear "you must re-upload your files" reminder. Gumroad's
//     `file_url` is a session-scoped link, not a downloadable URL we can hit
//     server-side, and even if it were we'd be exfiltrating files the user
//     already has on their machine.
//
// Build tag: !private_distribution — PrivateDistribution is the EXTERNAL_PAYMENT-only minimal build and doesn't
// need vendor migration tools.

const (
	gumroadAPIBase = "https://api.gumroad.com/v2"

	// Hard caps to keep imports predictable. Gumroad creators with thousands
	// of products are rare; if we ever need to lift this we'll add explicit
	// pagination. The /products endpoint returns the full catalogue in a
	// single response, so a cap also protects us against runaway memory.
	maxGumroadProductsPerImport   = 200
	maxGumroadThumbnailBytes      = 8 * 1024 * 1024 // 8 MiB per image
	maxGumroadConcurrentDownloads = 4
	gumroadHTTPTimeout            = 30 * time.Second
	gumroadOverallTimeout         = 5 * time.Minute
	gumroadMaxRequestBytes        = 1 << 20 // 1 MiB body cap (token + product IDs)

	gumroadFileUploadReminder = "Gumroad's protected file links can't be downloaded by third-party tools. " +
		"After import, open each draft listing and upload the actual digital file under " +
		"\"Digital Asset\" before publishing."
)

// gumroadImportRequest is the JSON body for the import endpoint. dryRun=true
// returns the would-be import plan without writing to the listing store.
type gumroadImportRequest struct {
	AccessToken string   `json:"accessToken"`
	DryRun      bool     `json:"dryRun"`
	ProductIDs  []string `json:"productIds,omitempty"` // nil/empty = all eligible products
	AsDraft     *bool    `json:"asDraft,omitempty"`    // default true
}

// gumroadImportPreviewItem is the per-product summary returned regardless of
// dry-run mode. UI uses it to render a checklist before the user confirms.
type gumroadImportPreviewItem struct {
	ExternalID     string   `json:"externalId"`
	Name           string   `json:"name"`
	PriceMinor     string   `json:"priceMinor"` // cents
	Currency       string   `json:"currency"`
	FormattedPrice string   `json:"formattedPrice,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	ThumbnailURL   string   `json:"thumbnailUrl,omitempty"`
	Published      bool     `json:"published"`
	WillImport     bool     `json:"willImport"`
	SkipReason     string   `json:"skipReason,omitempty"`
	MappedSlug     string   `json:"mappedSlug,omitempty"`
}

// gumroadImportResponse is the result of either a dry-run preview or the
// actual import. For non-dry-run calls, ImportResult is populated.
type gumroadImportResponse struct {
	TotalFetched       int                        `json:"totalFetched"`
	EligibleCount      int                        `json:"eligibleCount"`
	ImportedCount      int                        `json:"importedCount"`
	SkippedCount       int                        `json:"skippedCount"`
	FailedCount        int                        `json:"failedCount"`
	DryRun             bool                       `json:"dryRun"`
	Items              []gumroadImportPreviewItem `json:"items"`
	ImportResult       *ImportResult              `json:"importResult,omitempty"`
	FileUploadReminder string                     `json:"fileUploadReminder"`
	Warnings           []string                   `json:"warnings,omitempty"`
}

// gumroadAPIProduct mirrors the subset of the Gumroad v2 product object we
// care about. JSON decoder silently skips fields we don't list.
type gumroadAPIProduct struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"` // HTML
	Price              int64    `json:"price"`       // in cents (Gumroad always uses minor units)
	Currency           string   `json:"currency"`    // e.g. "usd"
	FormattedPrice     string   `json:"formatted_price,omitempty"`
	ShortURL           string   `json:"short_url,omitempty"`
	ThumbnailURL       string   `json:"thumbnail_url,omitempty"`
	PreviewURL         string   `json:"preview_url,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	Published          bool     `json:"published"`
	Deleted            bool     `json:"deleted,omitempty"`
	RequireShipping    bool     `json:"require_shipping,omitempty"`
	CustomizablePrice  bool     `json:"customizable_price,omitempty"`
	IsTieredMembership bool     `json:"is_tiered_membership,omitempty"`
}

type gumroadListProductsResponse struct {
	Success  bool                `json:"success"`
	Products []gumroadAPIProduct `json:"products"`
	Message  string              `json:"message,omitempty"`
}

// gumroadClient is a small typed wrapper around http.Client. Holding it in a
// struct lets the unit tests inject an httptest server URL.
type gumroadClient struct {
	baseURL string
	httpc   *http.Client
}

func newGumroadClient() *gumroadClient {
	return &gumroadClient{
		baseURL: gumroadAPIBase,
		httpc:   newGumroadHTTPClient(),
	}
}

// newGumroadHTTPClient builds the http.Client used for both the API and the
// thumbnail download path. It hardens against SSRF along three axes:
//
//  1. Reject non-public destination IPs at dial time via a Control hook on
//     net.Dialer. Tenant-attacker scenario: a hostile Gumroad token owner
//     could plant a `thumbnail_url` pointing at 169.254.169.254 (cloud
//     instance metadata) or a private VPC IP. With this hook the actual
//     TCP connection is refused before any HTTP request is sent.
//  2. Disallow HTTP redirects entirely — Gumroad's CDN never needs them
//     for thumbnails, and allowing redirects would let an attacker bypass
//     the URL-time scheme/host checks by redirecting from an https Gumroad
//     URL to http://169.254.169.254/.
//  3. Fixed timeout so a slowloris-style internal target can't hang a
//     worker.
//
// We do not currently allow-list Gumroad CDN domains because (a) Gumroad
// rotates their CDN host occasionally and (b) the dial-time IP filter is
// already a tight bound. If Gumroad introduces an unrelated third-party
// CDN we can revisit.
func newGumroadHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   ssrfSafeDialControl,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   gumroadHTTPTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ssrfSafeDialControl rejects the dial when the resolved IP is private,
// loopback, link-local, multicast, or unspecified. Runs after DNS resolution
// so DNS-rebinding attacks (CNAME / A-record swap mid-flight) are also
// caught — the kernel hands us the post-resolution IP here.
func ssrfSafeDialControl(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("ssrf guard: split host: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Should be unreachable — Control runs after DNS, address is IP.
		return fmt.Errorf("ssrf guard: cannot parse %q as IP", host)
	}
	if !isPublicIP(ip) {
		return fmt.Errorf("ssrf guard: refusing to dial non-public address %s", ip.String())
	}
	return nil
}

// cgnatNet is RFC 6598 carrier-grade NAT (100.64.0.0/10). Go's net.IP.IsPrivate
// covers RFC 1918 only, so we add CGNAT explicitly — these addresses appear
// on ISP-side NAT and are not routable on the public internet.
var cgnatNet = &net.IPNet{IP: net.IPv4(100, 64, 0, 0).To4(), Mask: net.CIDRMask(10, 32)}

// isPublicIP returns true when ip is routable on the public internet.
// Mirrors the rejection set used by major SSRF guards (ssrf_req_filter,
// safecurl, etc.) and explicitly covers IMDSv1 (169.254.169.254) via the
// link-local IPv4 range.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip.IsPrivate() {
		return false
	}
	if v4 := ip.To4(); v4 != nil && cgnatNet.Contains(v4) {
		return false
	}
	// IPv6 unique-local (fc00::/7): Go 1.17+ IsPrivate already covers it,
	// but keep an explicit check as a belt-and-braces against older runtimes.
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil {
		if v6[0]&0xfe == 0xfc {
			return false
		}
	}
	return true
}

// fetchProducts calls GET /v2/products. The Gumroad API does not paginate
// this endpoint — it returns the full catalogue in one response. We cap the
// returned slice to maxGumroadProductsPerImport to bound memory and report
// the original total back so the caller can warn the user about truncation.
func (c *gumroadClient) fetchProducts(ctx context.Context, accessToken string) (products []gumroadAPIProduct, totalAvailable int, err error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, 0, errors.New("access token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/products", nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("gumroad request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, 0, errors.New("invalid Gumroad access token (got 401/403)")
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, 0, errors.New("Gumroad rate limit hit; please retry in a minute")
	case resp.StatusCode >= 400:
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Message != "" {
			return nil, 0, fmt.Errorf("gumroad API error %d: %s", resp.StatusCode, apiErr.Message)
		}
		return nil, 0, fmt.Errorf("gumroad API returned %d", resp.StatusCode)
	}

	var parsed gumroadListProductsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, 0, fmt.Errorf("parse Gumroad response: %w", err)
	}
	if !parsed.Success && parsed.Message != "" {
		return nil, 0, fmt.Errorf("gumroad: %s", parsed.Message)
	}

	totalAvailable = len(parsed.Products)
	if totalAvailable > maxGumroadProductsPerImport {
		parsed.Products = parsed.Products[:maxGumroadProductsPerImport]
	}
	return parsed.Products, totalAvailable, nil
}

// downloadThumbnail fetches a single thumbnail with bounded size. Returns
// (nil, "", nil) — i.e. no error and no data — when the URL is empty.
// Returns ("", "", err) on retrievable errors so the caller can decide to
// continue without an image (which marks the product as "skip — no image").
//
// SSRF posture (defense in depth, see also newGumroadHTTPClient):
//   - URL-time: scheme must be https; embedded userinfo is rejected; only
//     well-formed absolute URLs accepted.
//   - Connect-time: ssrfSafeDialControl on the underlying dialer refuses
//     non-public IPs.
//   - Redirect-time: CheckRedirect returns ErrUseLastResponse so any 3xx is
//     treated as the final response (no follow). Coupled with the strict
//     scheme check this prevents an https→http or https→loopback bounce.
func (c *gumroadClient) downloadThumbnail(ctx context.Context, rawURL string) ([]byte, string, error) {
	if rawURL == "" {
		return nil, "", nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid thumbnail URL: %w", err)
	}
	if !parsed.IsAbs() {
		return nil, "", errors.New("thumbnail URL must be absolute")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return nil, "", fmt.Errorf("thumbnail URL must be https (got %q)", parsed.Scheme)
	}
	if parsed.User != nil {
		// Embedded credentials would survive into the outbound request and
		// can disguise an internal host (`http://attacker@127.0.0.1`).
		return nil, "", errors.New("thumbnail URL must not embed credentials")
	}
	if parsed.Host == "" {
		return nil, "", errors.New("thumbnail URL missing host")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// CheckRedirect = ErrUseLastResponse means the body is the redirect
		// response itself; treat as a fetch failure rather than silently
		// importing whatever the redirect-target host might want to send.
		return nil, "", fmt.Errorf("thumbnail fetch returned redirect status %d (not followed)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("thumbnail fetch returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGumroadThumbnailBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxGumroadThumbnailBytes {
		return nil, "", fmt.Errorf("thumbnail exceeds %d bytes", maxGumroadThumbnailBytes)
	}

	// Detect content type from magic bytes — Gumroad's CDN sometimes serves
	// without a Content-Type header, and we want a safe extension regardless.
	ext := "jpg"
	if kind, kerr := filetype.Match(data); kerr == nil && kind.Extension != "" {
		ext = kind.Extension
	}
	return data, ext, nil
}

// transformGumroadProduct maps a single Gumroad product into the JSON import
// schema used by processListingsImportJSON. Returns (nil, "skip reason") when
// the product cannot be imported.
//
// Decisions:
//   - Drop tiered memberships (Gumroad-specific recurring billing — we have
//     no equivalent today).
//   - Drop products with require_shipping (PHYSICAL_GOOD requires a shipping
//     profile picker, deferred to a later iteration).
//   - Drop "pay what you want" customizable_price products — we'd otherwise
//     import them at $0.
//   - Drop deleted products.
//   - Land everything else as DIGITAL_GOOD draft, with the thumbnail as the
//     only image. The seller adds the actual digital asset post-import.
func transformGumroadProduct(p gumroadAPIProduct, thumbnailFilename string, asDraft bool) (*JSONListingInput, string) {
	if p.Deleted {
		return nil, "product deleted on Gumroad"
	}
	if p.RequireShipping {
		return nil, "product requires shipping (physical goods import not yet supported)"
	}
	if p.IsTieredMembership {
		return nil, "tiered membership products are not supported"
	}
	if p.CustomizablePrice {
		return nil, `"pay what you want" pricing is not supported (set a fixed price on Gumroad first)`
	}
	if p.Price < 0 {
		return nil, "invalid price"
	}
	if p.Name == "" {
		return nil, "product has no name"
	}
	if thumbnailFilename == "" {
		return nil, "no usable thumbnail (Mobazha listings require at least one image)"
	}

	currency := strings.ToUpper(strings.TrimSpace(p.Currency))
	if currency == "" {
		currency = "USD"
	}

	// Gumroad ships price as cents; processListingsImportJSON expects a
	// human-readable price string (e.g. "9.50") that it then re-converts.
	major := float64(p.Price) / 100.0
	priceStr := fmt.Sprintf("%.2f", major)

	status := "draft"
	if !asDraft {
		status = "published"
	}

	tags := dedupeTrimNonEmpty(p.Tags)

	// Description: keep Gumroad's HTML — Mobazha's description field already
	// renders rich text, and stripping HTML server-side risks losing
	// formatting the seller deliberately added (links, bullets, code blocks).
	desc := p.Description

	return &JSONListingInput{
		Title:           p.Name,
		ContractType:    "DIGITAL_GOOD",
		Price:           priceStr,
		PricingCurrency: currency,
		Description:     desc,
		ProductType:     "Digital",
		Tags:            tags,
		Images:          []string{thumbnailFilename},
		Status:          status,
	}, ""
}

func dedupeTrimNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func sanitizeFilenameSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "x"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// gumroadProcessed is the intermediate result after fetch + transform. The
// handler reuses this for both dry-run preview and the actual import path so
// the work happens exactly once per request.
type gumroadProcessed struct {
	resp       *gumroadImportResponse
	inputs     []JSONListingInput
	thumbnails map[string][]byte
}

// processGumroadImport is the pure work function — fetch products, download
// thumbnails, transform, classify. It does not touch the listing store.
//
// Split out from the HTTP handler so unit tests can drive it with a stubbed
// gumroadClient (httptest server) and assert on the structured result.
func processGumroadImport(ctx context.Context, client *gumroadClient, req gumroadImportRequest) (*gumroadProcessed, error) {
	products, totalAvailable, err := client.fetchProducts(ctx, req.AccessToken)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if totalAvailable > maxGumroadProductsPerImport {
		// Honest disclosure rather than silent truncation. If a creator has
		// >200 products we want them to know they need to re-run with
		// productIds (or wait for paginated import in a future iteration).
		warnings = append(warnings, fmt.Sprintf(
			"Your Gumroad account has %d products; only the first %d were considered. Use the productIds filter to import the rest.",
			totalAvailable, maxGumroadProductsPerImport,
		))
	}

	if len(req.ProductIDs) > 0 {
		want := make(map[string]struct{}, len(req.ProductIDs))
		for _, id := range req.ProductIDs {
			want[id] = struct{}{}
		}
		filtered := products[:0]
		for _, p := range products {
			if _, ok := want[p.ID]; ok {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}

	asDraft := true
	if req.AsDraft != nil {
		asDraft = *req.AsDraft
	}

	// Cheap pre-filter: classify products that will be skipped regardless of
	// thumbnail availability so we don't waste network calls fetching images
	// for products we'll discard anyway.
	preSkip := make([]string, len(products))
	needsThumb := make([]bool, len(products))
	for i, p := range products {
		_, skip := transformGumroadProduct(p, "stub", asDraft)
		if skip != "" {
			preSkip[i] = skip
			continue
		}
		needsThumb[i] = true
	}

	thumbnails := make(map[string][]byte, len(products))
	thumbFilenames := make([]string, len(products))
	thumbErrors := make([]error, len(products))

	if len(products) > 0 {
		type job struct {
			idx int
			url string
			id  string
		}
		ch := make(chan job, len(products))
		results := make(chan struct {
			idx      int
			filename string
			data     []byte
			err      error
		}, len(products))

		var wg sync.WaitGroup
		workers := maxGumroadConcurrentDownloads
		if workers > len(products) {
			workers = len(products)
		}
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range ch {
					data, ext, derr := client.downloadThumbnail(ctx, j.url)
					if derr != nil || len(data) == 0 {
						results <- struct {
							idx      int
							filename string
							data     []byte
							err      error
						}{idx: j.idx, err: derr}
						continue
					}
					filename := fmt.Sprintf("gumroad-%s.%s", sanitizeFilenameSegment(j.id), ext)
					results <- struct {
						idx      int
						filename string
						data     []byte
						err      error
					}{idx: j.idx, filename: filename, data: data}
				}
			}()
		}
		for i, p := range products {
			if !needsThumb[i] {
				continue
			}
			thumbURL := p.ThumbnailURL
			if thumbURL == "" {
				thumbURL = p.PreviewURL
			}
			ch <- job{idx: i, url: thumbURL, id: p.ID}
		}
		close(ch)
		wg.Wait()
		close(results)
		for r := range results {
			if r.err != nil {
				thumbErrors[r.idx] = r.err
			}
			if r.filename != "" {
				thumbnails[r.filename] = r.data
				thumbFilenames[r.idx] = r.filename
			}
		}
	}

	resp := &gumroadImportResponse{
		TotalFetched:       len(products),
		DryRun:             req.DryRun,
		Items:              make([]gumroadImportPreviewItem, 0, len(products)),
		FileUploadReminder: gumroadFileUploadReminder,
		Warnings:           warnings,
	}

	var inputs []JSONListingInput
	for i, p := range products {
		item := gumroadImportPreviewItem{
			ExternalID:     p.ID,
			Name:           p.Name,
			PriceMinor:     fmt.Sprintf("%d", p.Price),
			Currency:       strings.ToUpper(p.Currency),
			FormattedPrice: p.FormattedPrice,
			Tags:           p.Tags,
			ThumbnailURL:   p.ThumbnailURL,
			Published:      p.Published,
		}
		// Honor the cheap-pre-skip decision (deleted/shipping/tiered/etc.) so
		// the per-product reason in the UI reflects the real cause rather
		// than the secondary "no thumbnail" symptom.
		if preSkip[i] != "" {
			item.WillImport = false
			item.SkipReason = preSkip[i]
			resp.Items = append(resp.Items, item)
			resp.SkippedCount++
			continue
		}
		filename := thumbFilenames[i]
		input, skip := transformGumroadProduct(p, filename, asDraft)
		if skip != "" {
			item.WillImport = false
			item.SkipReason = skip
			if filename == "" && thumbErrors[i] != nil {
				item.SkipReason = "thumbnail download failed: " + thumbErrors[i].Error()
			}
			resp.Items = append(resp.Items, item)
			resp.SkippedCount++
			continue
		}
		// Stable per-product slug so a re-run is idempotent (existing slugs
		// trigger the update path in processListingsImportJSON).
		input.Slug = fmt.Sprintf("gumroad-%s", sanitizeFilenameSegment(p.ID))
		item.WillImport = true
		item.MappedSlug = input.Slug
		resp.Items = append(resp.Items, item)
		resp.EligibleCount++
		inputs = append(inputs, *input)
	}

	// Sort items deterministically for the UI (eligible first, then skipped;
	// alphabetical within each group).
	sort.SliceStable(resp.Items, func(i, j int) bool {
		if resp.Items[i].WillImport != resp.Items[j].WillImport {
			return resp.Items[i].WillImport
		}
		return resp.Items[i].Name < resp.Items[j].Name
	})

	return &gumroadProcessed{
		resp:       resp,
		inputs:     inputs,
		thumbnails: thumbnails,
	}, nil
}

// handlePOSTGumroadImport is the chi handler exposed via Huma at
// POST /v1/listings/import/gumroad. The same endpoint handles both the
// dry-run preview (dryRun=true) and the actual import — clients gate on
// the `dryRun` field in the request body.
//
// We reuse processListingsImportJSON for the actual save path so the slug
// dedupe / image upload rules stay in one place. This means a re-run of the
// importer with the same Gumroad token behaves as an update — by design,
// since the seller may want to refresh pricing or descriptions after editing
// on Gumroad.
func (g *Gateway) handlePOSTGumroadImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, gumroadMaxRequestBytes))
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var req gumroadImportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(req.AccessToken) == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidation, "accessToken is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), gumroadOverallTimeout)
	defer cancel()

	processed, err := processGumroadImport(ctx, newGumroadClient(), req)
	if err != nil {
		// Bad gateway — the upstream Gumroad call (or thumbnail CDN) failed.
		// Most operator-facing causes here are bad token / rate limit /
		// transient outage; the message text is already user-friendly.
		response.Error(w, http.StatusBadGateway, response.CodeProviderError, err.Error())
		return
	}

	if !req.DryRun && len(processed.inputs) > 0 {
		listingSvc := getListingService(r)
		mediaSvc := getMediaService(r)
		ir, perr := g.processListingsImportJSON(listingSvc, mediaSvc, processed.inputs, processed.thumbnails, nil, nil)
		if perr != nil {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, perr.Error())
			return
		}
		processed.resp.ImportResult = ir
		processed.resp.ImportedCount = ir.Created + ir.Updated
		processed.resp.FailedCount = ir.Failed
	}

	response.Success(w, processed.resp)
}
