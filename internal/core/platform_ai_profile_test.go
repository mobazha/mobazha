//go:build !private_distribution

package core

import (
	"sync"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
)

func TestPlatformAIProfile_ConcurrentReadWrite(t *testing.T) {
	node := &MobazhaNode{}
	text := &aipkg.Config{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash", Enabled: true, IsPlatform: true}
	vision := &aipkg.Config{Provider: "qwen", APIKey: "vision-key", Model: "qwen3-vl-flash", Enabled: true, IsPlatform: true}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.SetPlatformAIProfile(aipkg.PlatformProfile{Text: text, Vision: vision})
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = node.PlatformAIConfig()
			_ = node.PlatformAIProfile()
		}()
	}
	wg.Wait()
}
