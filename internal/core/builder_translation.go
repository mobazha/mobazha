package core

import (
	"encoding/json"
	"strings"

	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// initTranslationSubsystem wires the node's content translation: it migrates
// the cache tables, builds the service and installs the engine the operator
// configured.
//
// A node with no engine configured still gets the service, so the interface it
// exposes is the same everywhere; every text simply comes back unchanged. That
// is the default because translating sends a seller's words to somebody else's
// server, which is the node owner's decision to make, not a default to inherit.
func initTranslationSubsystem(obNode *MobazhaNode, cfg *repo.Config) {
	if obNode == nil || obNode.db == nil {
		return
	}
	if err := database.MigrateTranslationModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "Translation: failed to migrate models: %v", err)
		return
	}

	service := NewTranslationAppService(database.NewGormTranslationStore(obNode.db))
	if cfg != nil {
		service.SetGlossary(parseTranslationGlossary(cfg.TranslationGlossary))
		if err := service.Configure(contracts.TranslationEngineConfig{
			Engine:            cfg.TranslationEngine,
			Endpoint:          cfg.TranslationEndpoint,
			APIKey:            cfg.TranslationAPIKey,
			Model:             cfg.TranslationModel,
			MonthlyCharBudget: cfg.TranslationMonthlyBudget,
		}); err != nil {
			// A bad engine name or a missing endpoint is a configuration
			// mistake, and it is reported at startup rather than on the first
			// visitor's page. The node keeps running, untranslated.
			logger.LogErrorWithIDf(log, obNode.nodeID, "Translation: %v (content will stay in its own language)", err)
			return
		}
	}

	obNode.translationService = service
	logger.LogInfoWithIDf(log, obNode.nodeID, "Translation subsystem initialized (engine=%s)",
		translationEngineName(service))
}

// parseTranslationGlossary reads the operator's term list. A malformed list is
// reported and ignored rather than taken as a reason to stop the node: the
// glossary improves quality, it is not what makes translation work.
func parseTranslationGlossary(raw string) []contracts.TranslationTerm {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var terms []contracts.TranslationTerm
	if err := json.Unmarshal([]byte(raw), &terms); err != nil {
		log.Warningf("Translation: the glossary is not a JSON array of {source,target}: %s", err)
		return nil
	}
	out := make([]contracts.TranslationTerm, 0, len(terms))
	for _, term := range terms {
		if strings.TrimSpace(term.Source) == "" {
			continue
		}
		out = append(out, term)
	}
	return out
}

// translationEngineName is what to log: the configured engine and model, or
// that nothing is configured.
func translationEngineName(service *TranslationAppService) string {
	if service == nil || !service.Enabled() {
		return contracts.TranslationEngineNone
	}
	return service.engineName()
}
