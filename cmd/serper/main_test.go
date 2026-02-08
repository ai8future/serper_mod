package main

import (
	"os"
	"testing"

	chassis "github.com/ai8future/chassis-go"
	chassisconfig "github.com/ai8future/chassis-go/config"
	"github.com/ai8future/chassis-go/testkit"
)

func TestMain(m *testing.M) {
	chassis.RequireMajor(4)
	code := m.Run()
	chassis.ResetVersionCheck()
	os.Exit(code)
}

func TestConfig_Defaults(t *testing.T) {
	testkit.SetEnv(t, map[string]string{
		"SERPER_API_KEY": "test-key",
	})

	cfg := chassisconfig.MustLoad[Config]()
	if cfg.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://google.serper.dev" {
		t.Errorf("expected default BaseURL, got %q", cfg.BaseURL)
	}
	if cfg.Num != 10 {
		t.Errorf("expected default Num 10, got %d", cfg.Num)
	}
	if cfg.GL != "us" {
		t.Errorf("expected default GL 'us', got %q", cfg.GL)
	}
}

func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
	testkit.SetEnv(t, map[string]string{})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing SERPER_API_KEY")
		}
	}()
	_ = chassisconfig.MustLoad[Config]()
}
