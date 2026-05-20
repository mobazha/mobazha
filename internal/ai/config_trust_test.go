package ai

import "testing"

func TestIsTrustedLocalLLMEndpoint_RejectsRemotePlainHTTP(t *testing.T) {
	if IsTrustedLocalLLMEndpoint("http://attacker.com/v1") {
		t.Fatal("remote plain HTTP endpoint must not be trusted without an API key")
	}
}

func TestIsTrustedLocalLLMEndpoint_AllowsLocalAndDockerInternalHTTP(t *testing.T) {
	tests := []string{
		"http://localhost:11434/v1",
		"http://127.0.0.1:11434/v1",
		"http://[::1]:11434/v1",
		"http://192.168.65.2:11434/v1",
		"http://host.docker.internal:11434/v1",
		"http://ollama:11434/v1",
	}
	for _, rawURL := range tests {
		if !IsTrustedLocalLLMEndpoint(rawURL) {
			t.Fatalf("%s should be trusted as local or Docker-internal", rawURL)
		}
	}
}

func TestIsTrustedLocalLLMEndpoint_RejectsRemoteHTTPSWithoutLocality(t *testing.T) {
	if IsTrustedLocalLLMEndpoint("https://api.openai.com/v1") {
		t.Fatal("remote HTTPS endpoint is not local/trusted for keyless use")
	}
}
