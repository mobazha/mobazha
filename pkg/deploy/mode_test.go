package deploy

import "testing"

func TestDefaultMode_IsStandalone(t *testing.T) {
	// Reset to default state for a clean test.
	processMode.Store(int32(Standalone))

	if got := GetProcessMode(); got != Standalone {
		t.Fatalf("default mode = %d, want Standalone (%d)", got, Standalone)
	}
	if IsSaaS() {
		t.Fatal("IsSaaS() should be false for default mode")
	}
}

func TestSetProcessMode_SaaS(t *testing.T) {
	processMode.Store(int32(Standalone)) // reset
	defer processMode.Store(int32(Standalone))

	SetProcessMode(SaaS)

	if got := GetProcessMode(); got != SaaS {
		t.Fatalf("mode = %d, want SaaS (%d)", got, SaaS)
	}
	if !IsSaaS() {
		t.Fatal("IsSaaS() should be true after SetProcessMode(SaaS)")
	}
}

func TestSetProcessMode_BackToStandalone(t *testing.T) {
	processMode.Store(int32(Standalone)) // reset
	defer processMode.Store(int32(Standalone))

	SetProcessMode(SaaS)
	SetProcessMode(Standalone)

	if got := GetProcessMode(); got != Standalone {
		t.Fatalf("mode = %d, want Standalone (%d)", got, Standalone)
	}
	if IsSaaS() {
		t.Fatal("IsSaaS() should be false after reverting to Standalone")
	}
}
