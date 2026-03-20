package api

import "strings"

// MaskSensitiveString Mask sensitive strings, showing only first 4 and last 4 characters
// Used to mask API Key, Secret Key, Private Key and other sensitive information
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		return "****" // String too short, hide everything
	}
	return s[:4] + "****" + s[length-4:]
}

// SanitizeModelConfigForLog sanitizes model configuration for log output.
func SanitizeModelConfigForLog(req UpdateModelConfigRequest) map[string]interface{} {
	safe := make(map[string]interface{}, len(req.Models))
	for modelID, cfg := range req.Models {
		safe[modelID] = map[string]interface{}{
			"enabled":           cfg.Enabled,
			"api_key":           MaskSensitiveString(cfg.APIKey),
			"custom_api_url":    cfg.CustomAPIURL,
			"custom_model_name": cfg.CustomModelName,
		}
	}
	return safe
}

// SanitizeExchangeConfigForLog sanitizes exchange configuration for log output.
func SanitizeExchangeConfigForLog(req UpdateExchangeConfigRequest) map[string]interface{} {
	safe := make(map[string]interface{}, len(req.Exchanges))
	for exchangeID, cfg := range req.Exchanges {
		safe[exchangeID] = sanitizeExchangeConfigFields(
			cfg.Enabled,
			cfg.APIKey,
			cfg.SecretKey,
			cfg.Passphrase,
			cfg.Testnet,
			cfg.HyperliquidWalletAddr,
			cfg.HyperliquidUnifiedAcct,
			cfg.AsterUser,
			cfg.AsterSigner,
			cfg.AsterPrivateKey,
			cfg.LighterWalletAddr,
			cfg.LighterPrivateKey,
			cfg.LighterAPIKeyPrivateKey,
			cfg.LighterAPIKeyIndex,
		)
	}
	return safe
}

// SanitizeCreateExchangeRequestForLog sanitizes exchange create payloads for log output.
func SanitizeCreateExchangeRequestForLog(req CreateExchangeRequest) map[string]interface{} {
	return map[string]interface{}{
		"exchange_type": req.ExchangeType,
		"account_name":  req.AccountName,
		"enabled":       req.Enabled,
		"config": sanitizeExchangeConfigFields(
			req.Enabled,
			req.APIKey,
			req.SecretKey,
			req.Passphrase,
			req.Testnet,
			req.HyperliquidWalletAddr,
			req.HyperliquidUnifiedAcct,
			req.AsterUser,
			req.AsterSigner,
			req.AsterPrivateKey,
			req.LighterWalletAddr,
			req.LighterPrivateKey,
			req.LighterAPIKeyPrivateKey,
			req.LighterAPIKeyIndex,
		),
	}
}

func sanitizeExchangeConfigFields(enabled bool, apiKey, secretKey, passphrase string, testnet bool, hyperliquidWalletAddr string, hyperliquidUnifiedAcct bool, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey string, lighterAPIKeyIndex int) map[string]interface{} {
	safeExchange := map[string]interface{}{
		"enabled":                     enabled,
		"testnet":                     testnet,
		"hyperliquid_unified_account": hyperliquidUnifiedAcct,
	}

	if apiKey != "" {
		safeExchange["api_key"] = MaskSensitiveString(apiKey)
	}
	if secretKey != "" {
		safeExchange["secret_key"] = MaskSensitiveString(secretKey)
	}
	if passphrase != "" {
		safeExchange["passphrase"] = MaskSensitiveString(passphrase)
	}
	if asterPrivateKey != "" {
		safeExchange["aster_private_key"] = MaskSensitiveString(asterPrivateKey)
	}
	if lighterPrivateKey != "" {
		safeExchange["lighter_private_key"] = MaskSensitiveString(lighterPrivateKey)
	}
	if lighterAPIKeyPrivateKey != "" {
		safeExchange["lighter_api_key_private_key"] = MaskSensitiveString(lighterAPIKeyPrivateKey)
	}
	if hyperliquidWalletAddr != "" {
		safeExchange["hyperliquid_wallet_addr"] = hyperliquidWalletAddr
	}
	if asterUser != "" {
		safeExchange["aster_user"] = asterUser
	}
	if asterSigner != "" {
		safeExchange["aster_signer"] = asterSigner
	}
	if lighterWalletAddr != "" {
		safeExchange["lighter_wallet_addr"] = lighterWalletAddr
	}
	if lighterAPIKeyIndex > 0 {
		safeExchange["lighter_api_key_index"] = lighterAPIKeyIndex
	}

	return safeExchange
}

// MaskEmail Mask email address, keeping first 2 characters and domain part
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****" // Incorrect format
	}
	username := parts[0]
	domain := parts[1]
	if len(username) <= 2 {
		return "**@" + domain
	}
	return username[:2] + "****@" + domain
}
