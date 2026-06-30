//go:build !private_distribution

package core

import (
	"sync"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

func TestPlatformAIProfile_ConcurrentReadWrite(t *testing.T) {
	node := &MobazhaNode{}
	profile := contracts.AIProfile{
		Text:   contracts.AIEndpointConfig{Provider: "deepseek", APIKey: "text-key", Model: "deepseek-v4-flash"},
		Vision: contracts.AIEndpointConfig{Provider: "qwen", APIKey: "vision-key", Model: "qwen3-vl-flash"},
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.SetAIProfile(profile)
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
