package core

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const (
	maxEventsPerBatch    = 20
	maxEventAgeForStats  = 90 // days
	topPagesLimit        = 20
	topReferrersLimit    = 10
	maxEventsPerTenant   = 100_000
	cleanupOlderThanDays = 120
	cleanupInterval      = 24 * time.Hour
)

// AnalyticsAppService handles visitor event tracking and stats aggregation.
type AnalyticsAppService struct {
	db       database.Database
	nodeID   string
	shutdown <-chan struct{}
}

// AnalyticsAppServiceConfig holds dependencies for AnalyticsAppService.
type AnalyticsAppServiceConfig struct {
	DB       database.Database
	NodeID   string
	Shutdown <-chan struct{}
}

// NewAnalyticsAppService creates a new AnalyticsAppService and starts background cleanup.
func NewAnalyticsAppService(cfg AnalyticsAppServiceConfig) *AnalyticsAppService {
	s := &AnalyticsAppService{
		db:       cfg.DB,
		nodeID:   cfg.NodeID,
		shutdown: cfg.Shutdown,
	}
	if cfg.Shutdown != nil {
		go s.cleanupLoop()
	}
	return s
}

func (s *AnalyticsAppService) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			if err := s.Cleanup(); err != nil {
				logger.LogErrorWithIDf(log, s.nodeID, "analytics cleanup: %v", err)
			}
		}
	}
}

// TrackEvent persists a single analytics event after validation.
func (s *AnalyticsAppService) TrackEvent(evt models.AnalyticsEvent) error {
	if err := validateEvent(&evt); err != nil {
		return err
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Save(&evt)
	})
}

// TrackEvents persists a batch of analytics events in a single transaction.
func (s *AnalyticsAppService) TrackEvents(events []models.AnalyticsEvent) error {
	if len(events) > maxEventsPerBatch {
		events = events[:maxEventsPerBatch]
	}
	valid := make([]models.AnalyticsEvent, 0, len(events))
	for i := range events {
		if err := validateEvent(&events[i]); err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "analytics: skip event %d: %v", i, err)
			continue
		}
		valid = append(valid, events[i])
	}
	if len(valid) == 0 {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		for i := range valid {
			if err := tx.Save(&valid[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateEvent(evt *models.AnalyticsEvent) error {
	if !models.ValidEventTypes[evt.EventType] {
		return fmt.Errorf("invalid event type: %s", evt.EventType)
	}
	if len(evt.SessionID) == 0 || len(evt.SessionID) > 64 {
		return fmt.Errorf("sessionId must be 1-64 characters")
	}
	if len(evt.VisitorID) == 0 || len(evt.VisitorID) > 64 {
		return fmt.Errorf("visitorId must be 1-64 characters")
	}
	if len(evt.PagePath) > 512 {
		evt.PagePath = evt.PagePath[:512]
	}
	if len(evt.ProductSlug) > 256 {
		evt.ProductSlug = evt.ProductSlug[:256]
	}
	if len(evt.Referrer) > 512 {
		evt.Referrer = evt.Referrer[:512]
	}
	return nil
}

// VisitorSummary holds aggregate visitor metrics.
type VisitorSummary struct {
	TotalPageViews    int64 `json:"totalPageViews"`
	TotalProductViews int64 `json:"totalProductViews"`
	TotalAddToCart    int64 `json:"totalAddToCart"`
	TotalCheckoutStart int64 `json:"totalCheckoutStart"`
	UniqueVisitors    int64 `json:"uniqueVisitors"`
}

// VisitorTrendPoint holds a single day's aggregated visitor data.
type VisitorTrendPoint struct {
	Date     string `json:"date"`
	Visitors int64  `json:"visitors"`
	Views    int64  `json:"views"`
}

// PageStat holds view count for a single page.
type PageStat struct {
	Path     string `json:"path"`
	Views    int64  `json:"views"`
	Visitors int64  `json:"visitors"`
}

// ReferrerStat holds visit count from a referrer domain.
type ReferrerStat struct {
	Source string `json:"source"`
	Visits int64  `json:"visits"`
}

// ConversionFunnel holds counts per stage.
type ConversionFunnel struct {
	PageView      int64 `json:"pageView"`
	ProductView   int64 `json:"productView"`
	AddToCart     int64 `json:"addToCart"`
	CheckoutStart int64 `json:"checkoutStart"`
}

// VisitorStats combines all visitor analytics for the dashboard.
type VisitorStats struct {
	Summary   VisitorSummary      `json:"summary"`
	Trend     []VisitorTrendPoint `json:"trend"`
	TopPages  []PageStat          `json:"topPages"`
	Referrers []ReferrerStat      `json:"topReferrers"`
	Funnel    ConversionFunnel    `json:"funnel"`
}

// GetVisitorStats computes aggregated visitor statistics for the given period.
func (s *AnalyticsAppService) GetVisitorStats(days int) (any, error) {
	if days <= 0 || days > maxEventAgeForStats {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	var stats VisitorStats

	err := s.db.View(func(tx database.Tx) error {
		base := tx.Read().Model(&models.AnalyticsEvent{}).Where("created_at >= ?", cutoff)

		// Summary counts per event type
		type typeCount struct {
			EventType string
			Count     int64
		}
		var typeCounts []typeCount
		if err := base.Select("event_type, count(*) as count").
			Group("event_type").Scan(&typeCounts).Error; err != nil {
			return fmt.Errorf("count by type: %w", err)
		}
		for _, tc := range typeCounts {
			switch tc.EventType {
			case models.EventTypePageView:
				stats.Summary.TotalPageViews = tc.Count
			case models.EventTypeProductView:
				stats.Summary.TotalProductViews = tc.Count
			case models.EventTypeAddToCart:
				stats.Summary.TotalAddToCart = tc.Count
			case models.EventTypeCheckoutStart:
				stats.Summary.TotalCheckoutStart = tc.Count
			}
		}

		stats.Funnel = ConversionFunnel{
			PageView:      stats.Summary.TotalPageViews + stats.Summary.TotalProductViews,
			ProductView:   stats.Summary.TotalProductViews,
			AddToCart:     stats.Summary.TotalAddToCart,
			CheckoutStart: stats.Summary.TotalCheckoutStart,
		}

		// Unique visitors
		var uniqueCount int64
		if err := base.Distinct("visitor_id").Count(&uniqueCount).Error; err != nil {
			return fmt.Errorf("unique visitors: %w", err)
		}
		stats.Summary.UniqueVisitors = uniqueCount

		// Daily trend
		type dayRow struct {
			Day      string
			Visitors int64
			Views    int64
		}
		var dayRows []dayRow
		if err := tx.Read().Model(&models.AnalyticsEvent{}).
			Where("created_at >= ?", cutoff).
			Select("DATE(created_at) as day, COUNT(DISTINCT visitor_id) as visitors, COUNT(*) as views").
			Group("day").Order("day").
			Scan(&dayRows).Error; err != nil {
			return fmt.Errorf("trend: %w", err)
		}

		dayMap := make(map[string]VisitorTrendPoint, days)
		for i := 0; i < days; i++ {
			d := time.Now().AddDate(0, 0, -days+1+i)
			key := d.Format("2006-01-02")
			dayMap[key] = VisitorTrendPoint{Date: key}
		}
		for _, r := range dayRows {
			if pt, ok := dayMap[r.Day]; ok {
				pt.Visitors = r.Visitors
				pt.Views = r.Views
				dayMap[r.Day] = pt
			}
		}
		stats.Trend = make([]VisitorTrendPoint, 0, days)
		for i := 0; i < days; i++ {
			d := time.Now().AddDate(0, 0, -days+1+i)
			key := d.Format("2006-01-02")
			stats.Trend = append(stats.Trend, dayMap[key])
		}

		// Top pages
		type pageRow struct {
			PagePath string
			Views    int64
			Visitors int64
		}
		var pageRows []pageRow
		if err := tx.Read().Model(&models.AnalyticsEvent{}).
			Where("created_at >= ? AND page_path != ''", cutoff).
			Select("page_path, COUNT(*) as views, COUNT(DISTINCT visitor_id) as visitors").
			Group("page_path").Order("views DESC").
			Limit(topPagesLimit).
			Scan(&pageRows).Error; err != nil {
			return fmt.Errorf("top pages: %w", err)
		}
		stats.TopPages = make([]PageStat, 0, len(pageRows))
		for _, r := range pageRows {
			stats.TopPages = append(stats.TopPages, PageStat{
				Path:     r.PagePath,
				Views:    r.Views,
				Visitors: r.Visitors,
			})
		}

		// Top referrers (extract domain)
		type refRow struct {
			Referrer string
			Visits   int64
		}
		var refRows []refRow
		if err := tx.Read().Model(&models.AnalyticsEvent{}).
			Where("created_at >= ? AND referrer != ''", cutoff).
			Select("referrer, COUNT(*) as visits").
			Group("referrer").Order("visits DESC").
			Limit(50).
			Scan(&refRows).Error; err != nil {
			return fmt.Errorf("referrers: %w", err)
		}
		domainMap := make(map[string]int64)
		for _, r := range refRows {
			domain := extractDomain(r.Referrer)
			domainMap[domain] += r.Visits
		}
		stats.Referrers = make([]ReferrerStat, 0, len(domainMap))
		for domain, visits := range domainMap {
			stats.Referrers = append(stats.Referrers, ReferrerStat{
				Source: domain,
				Visits: visits,
			})
		}
		sort.Slice(stats.Referrers, func(i, j int) bool {
			return stats.Referrers[i].Visits > stats.Referrers[j].Visits
		})
		if len(stats.Referrers) > topReferrersLimit {
			stats.Referrers = stats.Referrers[:topReferrersLimit]
		}

		return nil
	})
	if err != nil {
		logger.LogErrorWithIDf(log, s.nodeID, "analytics stats: %v", err)
		return nil, err
	}

	return &stats, nil
}

// Cleanup removes old analytics events beyond the retention window.
func (s *AnalyticsAppService) Cleanup() error {
	cutoff := time.Now().AddDate(0, 0, -cleanupOlderThanDays)
	return s.db.Update(func(tx database.Tx) error {
		return tx.Delete("created_at <= ?", cutoff, nil, &models.AnalyticsEvent{})
	})
}

func extractDomain(ref string) string {
	if ref == "" {
		return "(direct)"
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return ref
	}
	return u.Host
}
