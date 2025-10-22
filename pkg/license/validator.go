package license

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// License represents a parsed and validated license
type License struct {
	// Standard JWT claims
	Issuer     string    `json:"iss"`
	Subject    string    `json:"sub"`
	IssuedAt   time.Time `json:"iat"`
	ExpiresAt  time.Time `json:"exp"`
	NotBefore  time.Time `json:"nbf"`

	// Custom claims
	LicenseID       string            `json:"license_id"`
	CustomerID      string            `json:"customer_id"`
	CustomerName    string            `json:"customer_name"`
	ProductCode     string            `json:"product_code"`
	ProductName     string            `json:"product_name"`
	TierCode        string            `json:"tier_code"`
	TierName        string            `json:"tier_name"`
	ClusterID       string            `json:"cluster_id"`
	ClusterName     string            `json:"cluster_name"`
	Namespace       string            `json:"namespace"`
	LicensedNodes   int               `json:"licensed_nodes"`
	MaxNodes        int               `json:"max_nodes,omitempty"`
	NodeSelector    map[string]string `json:"node_selector"`
	Features        []string          `json:"features"`
	GracePeriodDays int               `json:"grace_period_days"`
	WarningDays     int               `json:"warning_days"`
	PhoneHomeConfig PhoneHomeConfig   `json:"phone_home"`
}

// PhoneHomeConfig holds phone home configuration from the license
type PhoneHomeConfig struct {
	Enabled       bool   `json:"enabled"`
	URL           string `json:"url"`
	IntervalHours int    `json:"interval_hours"`
}

// ValidationResult represents the result of license validation
type ValidationResult struct {
	Valid            bool
	License          *License
	Error            error
	ExpiresAt        time.Time
	DaysUntilExpiry  int
	IsInGracePeriod  bool
	NodeCount        int
	LicensedNodes    int
	NodeCountValid   bool
	NamespaceValid   bool
	ActualNamespace  string
	LicenseNamespace string
	SignatureValid   bool
	ExpiryValid      bool
	ValidationTime   time.Time
}

// Validator validates license JWTs
type Validator struct {
	publicKey *rsa.PublicKey
}

// NewValidator creates a new license validator with the given public key
func NewValidator(publicKeyPEM string) (*Validator, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return &Validator{
		publicKey: rsaPub,
	}, nil
}

// Validate validates a license JWT and returns the validation result
func (v *Validator) Validate(licenseJWT string, actualNodeCount int, actualNamespace string) *ValidationResult {
	result := &ValidationResult{
		ValidationTime:  time.Now(),
		NodeCount:       actualNodeCount,
		ActualNamespace: actualNamespace,
	}

	// Parse and validate JWT
	token, err := jwt.ParseWithClaims(licenseJWT, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil {
		result.Error = fmt.Errorf("JWT validation failed: %w", err)
		result.Valid = false
		return result
	}

	result.SignatureValid = token.Valid

	// Extract claims
	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		result.Error = fmt.Errorf("invalid JWT claims")
		result.Valid = false
		return result
	}

	// Parse license from claims
	license, err := parseLicense(claims)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse license: %w", err)
		result.Valid = false
		return result
	}

	result.License = license
	result.ExpiresAt = license.ExpiresAt
	result.LicensedNodes = license.LicensedNodes
	result.LicenseNamespace = license.Namespace

	// Check expiration
	now := time.Now()
	result.DaysUntilExpiry = int(license.ExpiresAt.Sub(now).Hours() / 24)
	result.ExpiryValid = now.Before(license.ExpiresAt)

	// Check if in grace period
	gracePeriodEnd := license.ExpiresAt.AddDate(0, 0, license.GracePeriodDays)
	result.IsInGracePeriod = now.After(license.ExpiresAt) && now.Before(gracePeriodEnd)

	// Check node count
	result.NodeCountValid = actualNodeCount <= license.LicensedNodes

	// Check namespace match
	result.NamespaceValid = actualNamespace == license.Namespace

	// Overall validity: signature must be valid, not expired (or in grace period),
	// node count must be valid, AND namespace must match
	result.Valid = result.SignatureValid && (result.ExpiryValid || result.IsInGracePeriod) && result.NodeCountValid && result.NamespaceValid

	if !result.NamespaceValid {
		result.Error = fmt.Errorf("namespace mismatch: license is for namespace '%s' but validator is running in '%s'", license.Namespace, actualNamespace)
		result.Valid = false
	}

	return result
}

// parseLicense parses license claims into a License struct
func parseLicense(claims *jwt.MapClaims) (*License, error) {
	license := &License{}

	// Standard claims
	if iss, ok := (*claims)["iss"].(string); ok {
		license.Issuer = iss
	}
	if sub, ok := (*claims)["sub"].(string); ok {
		license.Subject = sub
	}
	if iat, ok := (*claims)["iat"].(float64); ok {
		license.IssuedAt = time.Unix(int64(iat), 0)
	}
	if exp, ok := (*claims)["exp"].(float64); ok {
		license.ExpiresAt = time.Unix(int64(exp), 0)
	}
	if nbf, ok := (*claims)["nbf"].(float64); ok {
		license.NotBefore = time.Unix(int64(nbf), 0)
	}

	// Custom claims
	if licenseID, ok := (*claims)["license_id"].(string); ok {
		license.LicenseID = licenseID
	}
	if customerID, ok := (*claims)["customer_id"].(string); ok {
		license.CustomerID = customerID
	}
	if customerName, ok := (*claims)["customer_name"].(string); ok {
		license.CustomerName = customerName
	}
	if productCode, ok := (*claims)["product_code"].(string); ok {
		license.ProductCode = productCode
	}
	if productName, ok := (*claims)["product_name"].(string); ok {
		license.ProductName = productName
	}
	if tierCode, ok := (*claims)["tier_code"].(string); ok {
		license.TierCode = tierCode
	}
	if tierName, ok := (*claims)["tier_name"].(string); ok {
		license.TierName = tierName
	}
	if clusterID, ok := (*claims)["cluster_id"].(string); ok {
		license.ClusterID = clusterID
	}
	if clusterName, ok := (*claims)["cluster_name"].(string); ok {
		license.ClusterName = clusterName
	}
	if namespace, ok := (*claims)["namespace"].(string); ok {
		license.Namespace = namespace
	}
	if licensedNodes, ok := (*claims)["licensed_nodes"].(float64); ok {
		license.LicensedNodes = int(licensedNodes)
	}
	if maxNodes, ok := (*claims)["max_nodes"].(float64); ok {
		license.MaxNodes = int(maxNodes)
	}
	if gracePeriodDays, ok := (*claims)["grace_period_days"].(float64); ok {
		license.GracePeriodDays = int(gracePeriodDays)
	}
	if warningDays, ok := (*claims)["warning_days"].(float64); ok {
		license.WarningDays = int(warningDays)
	}

	// Node selector
	if nodeSelector, ok := (*claims)["node_selector"].(map[string]interface{}); ok {
		license.NodeSelector = make(map[string]string)
		for k, v := range nodeSelector {
			if str, ok := v.(string); ok {
				license.NodeSelector[k] = str
			}
		}
	}

	// Features
	if features, ok := (*claims)["features"].([]interface{}); ok {
		license.Features = make([]string, 0, len(features))
		for _, f := range features {
			if str, ok := f.(string); ok {
				license.Features = append(license.Features, str)
			}
		}
	}

	// Phone home config
	if phoneHome, ok := (*claims)["phone_home"].(map[string]interface{}); ok {
		if enabled, ok := phoneHome["enabled"].(bool); ok {
			license.PhoneHomeConfig.Enabled = enabled
		}
		if url, ok := phoneHome["url"].(string); ok {
			license.PhoneHomeConfig.URL = url
		}
		if interval, ok := phoneHome["interval_hours"].(float64); ok {
			license.PhoneHomeConfig.IntervalHours = int(interval)
		}
	}

	return license, nil
}
