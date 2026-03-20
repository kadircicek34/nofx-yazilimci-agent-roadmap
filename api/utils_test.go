package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"nofx/config"

	"github.com/gin-gonic/gin"
)

func TestMaskSensitiveString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Empty string", input: "", expected: ""},
		{name: "Short string", input: "short", expected: "****"},
		{name: "Normal API key", input: "sk-1234567890abcdefghijklmnopqrstuvwxyz", expected: "sk-1****wxyz"},
		{name: "Normal private key", input: "0x1234567890abcdef1234567890abcdef12345678", expected: "0x12****5678"},
		{name: "Exactly 9 characters", input: "123456789", expected: "1234****6789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskSensitiveString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeModelConfigForLog(t *testing.T) {
	req := UpdateModelConfigRequest{Models: map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	}{
		"deepseek": {
			Enabled:         true,
			APIKey:          "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			CustomAPIURL:    "https://api.deepseek.com",
			CustomModelName: "deepseek-chat",
		},
	}}

	result := SanitizeModelConfigForLog(req)
	deepseekConfig, ok := result["deepseek"].(map[string]interface{})
	if !ok {
		t.Fatal("deepseek config not found or wrong type")
	}
	if deepseekConfig["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", deepseekConfig["enabled"])
	}
	if maskedKey, _ := deepseekConfig["api_key"].(string); maskedKey != "sk-1****wxyz" {
		t.Errorf("expected masked api_key='sk-1****wxyz', got %q", maskedKey)
	}
	if deepseekConfig["custom_api_url"] != "https://api.deepseek.com" {
		t.Errorf("custom_api_url should not be masked")
	}
}

func TestSanitizeExchangeConfigForLog(t *testing.T) {
	req := UpdateExchangeConfigRequest{Exchanges: map[string]struct {
		Enabled                 bool   `json:"enabled"`
		APIKey                  string `json:"api_key"`
		SecretKey               string `json:"secret_key"`
		Passphrase              string `json:"passphrase"`
		Testnet                 bool   `json:"testnet"`
		HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
		HyperliquidUnifiedAcct  bool   `json:"hyperliquid_unified_account"`
		AsterUser               string `json:"aster_user"`
		AsterSigner             string `json:"aster_signer"`
		AsterPrivateKey         string `json:"aster_private_key"`
		LighterWalletAddr       string `json:"lighter_wallet_addr"`
		LighterPrivateKey       string `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
		LighterAPIKeyIndex      int    `json:"lighter_api_key_index"`
	}{
		"binance": {
			Enabled:    true,
			APIKey:     "binance_api_key_1234567890abcdef",
			SecretKey:  "binance_secret_key_1234567890abcdef",
			Passphrase: "super-secret-passphrase",
		},
		"hyperliquid": {
			Enabled:               true,
			HyperliquidWalletAddr: "0x1234567890abcdef1234567890abcdef12345678",
		},
	}}

	result := SanitizeExchangeConfigForLog(req)
	binanceConfig, ok := result["binance"].(map[string]interface{})
	if !ok {
		t.Fatal("binance config not found or wrong type")
	}
	if maskedAPIKey, _ := binanceConfig["api_key"].(string); maskedAPIKey != "bina****cdef" {
		t.Errorf("expected masked api_key='bina****cdef', got %q", maskedAPIKey)
	}
	if maskedSecretKey, _ := binanceConfig["secret_key"].(string); maskedSecretKey != "bina****cdef" {
		t.Errorf("expected masked secret_key='bina****cdef', got %q", maskedSecretKey)
	}
	if maskedPassphrase, _ := binanceConfig["passphrase"].(string); maskedPassphrase == "super-secret-passphrase" || maskedPassphrase == "" {
		t.Errorf("expected passphrase to be masked, got %q", maskedPassphrase)
	}

	hlConfig, ok := result["hyperliquid"].(map[string]interface{})
	if !ok {
		t.Fatal("hyperliquid config not found or wrong type")
	}
	if walletAddr, _ := hlConfig["hyperliquid_wallet_addr"].(string); walletAddr != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("wallet address should not be masked, got %q", walletAddr)
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Empty email", input: "", expected: ""},
		{name: "Invalid format", input: "notanemail", expected: "****"},
		{name: "Normal email", input: "user@example.com", expected: "us****@example.com"},
		{name: "Short username", input: "a@example.com", expected: "**@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskEmail(tt.input)
			if result != tt.expected {
				t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHandleResetPassword_DisabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldValue, hadValue := os.LookupEnv("ENABLE_PUBLIC_PASSWORD_RESET")
	defer func() {
		if hadValue {
			_ = os.Setenv("ENABLE_PUBLIC_PASSWORD_RESET", oldValue)
		} else {
			_ = os.Unsetenv("ENABLE_PUBLIC_PASSWORD_RESET")
		}
		config.Init()
	}()
	_ = os.Unsetenv("ENABLE_PUBLIC_PASSWORD_RESET")
	config.Init()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/reset-password", strings.NewReader(`{"email":"user@example.com","new_password":"Secret123!"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	server := &Server{}
	server.handleResetPassword(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(body["error"], "disabled") {
		t.Fatalf("expected disabled error message, got %q", body["error"])
	}
}
