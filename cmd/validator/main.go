package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enterprisesight/es-license-validator/pkg/config"
	"github.com/enterprisesight/es-license-validator/pkg/license"
	"github.com/enterprisesight/es-license-validator/pkg/nodes"
	"github.com/enterprisesight/es-license-validator/pkg/phonehome"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PublicKey is the ES public key for JWT verification
// This will be embedded in the container or mounted as a ConfigMap
const DefaultPublicKey = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA...
-----END PUBLIC KEY-----`

type ValidatorService struct {
	cfg              *config.Config
	validator        *license.Validator
	nodeCounter      *nodes.Counter
	phoneHomeClient  *phonehome.Client
	currentResult    *license.ValidationResult
	k8sClient        *kubernetes.Clientset
}

func main() {
	log.Println("Starting ES License Validator...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Load public key
	publicKey := os.Getenv("ES_PUBLIC_KEY")
	if publicKey == "" {
		publicKey = DefaultPublicKey
		log.Println("Using default public key")
	}

	// Create validator
	validator, err := license.NewValidator(publicKey)
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}

	// Create node counter
	nodeCounter, err := nodes.NewCounter(cfg.NodeLabelKey, cfg.NodeLabelValue)
	if err != nil {
		log.Fatalf("Failed to create node counter: %v", err)
	}

	// Create phone home client
	var phoneHomeClient *phonehome.Client
	if cfg.PhoneHomeEnabled {
		phoneHomeClient = phonehome.NewClient(
			cfg.LicenseServerURL,
			cfg.PhoneHomeTimeout,
			cfg.PhoneHomeRetries,
		)
	}

	// Create Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	// Create service
	svc := &ValidatorService{
		cfg:             cfg,
		validator:       validator,
		nodeCounter:     nodeCounter,
		phoneHomeClient: phoneHomeClient,
		k8sClient:       k8sClient,
	}

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", svc.healthHandler)
	mux.HandleFunc("/ready", svc.readyHandler)
	mux.HandleFunc("/status", svc.statusHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	// Start validation loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.validationLoop(ctx)

	// Start server
	go func() {
		log.Printf("HTTP server listening on :%d", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}

func (s *ValidatorService) validationLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.ValidationInterval)
	defer ticker.Stop()

	// Run immediately on startup
	s.runValidation(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runValidation(ctx)
		}
	}
}

func (s *ValidatorService) runValidation(ctx context.Context) {
	log.Println("Running license validation...")

	// Read license from secret
	secret, err := s.k8sClient.CoreV1().Secrets(s.cfg.LicenseSecretNamespace).Get(
		ctx,
		s.cfg.LicenseSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		log.Printf("ERROR: Failed to read license secret: %v", err)
		s.currentResult = &license.ValidationResult{
			Valid:          false,
			Error:          fmt.Errorf("failed to read license secret: %w", err),
			ValidationTime: time.Now(),
		}
		return
	}

	licenseJWT, ok := secret.Data[s.cfg.LicenseSecretKey]
	if !ok {
		log.Printf("ERROR: License key '%s' not found in secret", s.cfg.LicenseSecretKey)
		s.currentResult = &license.ValidationResult{
			Valid:          false,
			Error:          fmt.Errorf("license key not found in secret"),
			ValidationTime: time.Now(),
		}
		return
	}

	// Count labeled nodes
	nodeCount, err := s.nodeCounter.CountLabeledNodes(ctx)
	if err != nil {
		log.Printf("ERROR: Failed to count nodes: %v", err)
		nodeCount = 0
	}

	// Validate license (including namespace binding check)
	result := s.validator.Validate(string(licenseJWT), nodeCount, s.cfg.LicenseSecretNamespace)
	s.currentResult = result

	// Log result
	if result.Valid {
		log.Printf("✓ License is VALID - Nodes: %d/%d, Expires in %d days",
			result.NodeCount, result.LicensedNodes, result.DaysUntilExpiry)
	} else if result.IsInGracePeriod {
		log.Printf("⚠ License EXPIRED but in GRACE PERIOD - Nodes: %d/%d",
			result.NodeCount, result.LicensedNodes)
	} else {
		log.Printf("✗ License is INVALID - %v", result.Error)
	}

	// Phone home if enabled
	if s.cfg.PhoneHomeEnabled && s.phoneHomeClient != nil && result.License != nil {
		go func() {
			phoneCtx, cancel := context.WithTimeout(context.Background(), s.cfg.PhoneHomeTimeout)
			defer cancel()

			if err := s.phoneHomeClient.SendPhoneHome(phoneCtx, result); err != nil {
				log.Printf("Phone home failed (fail-open): %v", err)
			} else {
				log.Println("Phone home successful")
			}
		}()
	}
}

func (s *ValidatorService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *ValidatorService) readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.currentResult == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "not_ready",
			"message": "No validation result yet",
		})
		return
	}

	if s.currentResult.Valid || (s.cfg.FailOpen && s.currentResult.IsInGracePeriod) {
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ready",
		})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "not_ready",
			"message": "License validation failed",
			"valid":   s.currentResult.Valid,
		})
	}
}

func (s *ValidatorService) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.currentResult == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "no_validation_result",
			"message": "Validation has not run yet",
		})
		return
	}

	response := map[string]interface{}{
		"valid":              s.currentResult.Valid,
		"validation_time":    s.currentResult.ValidationTime.Format(time.RFC3339),
		"node_count":         s.currentResult.NodeCount,
		"licensed_nodes":     s.currentResult.LicensedNodes,
		"days_until_expiry":  s.currentResult.DaysUntilExpiry,
		"in_grace_period":    s.currentResult.IsInGracePeriod,
		"signature_valid":    s.currentResult.SignatureValid,
		"expiry_valid":       s.currentResult.ExpiryValid,
		"node_count_valid":   s.currentResult.NodeCountValid,
		"namespace_valid":    s.currentResult.NamespaceValid,
		"actual_namespace":   s.currentResult.ActualNamespace,
		"license_namespace":  s.currentResult.LicenseNamespace,
	}

	if s.currentResult.License != nil {
		response["license"] = map[string]interface{}{
			"license_id":    s.currentResult.License.LicenseID,
			"customer_name": s.currentResult.License.CustomerName,
			"product_code":  s.currentResult.License.ProductCode,
			"product_name":  s.currentResult.License.ProductName,
			"tier_code":     s.currentResult.License.TierCode,
			"cluster_id":    s.currentResult.License.ClusterID,
			"namespace":     s.currentResult.License.Namespace,
			"expires_at":    s.currentResult.License.ExpiresAt.Format(time.RFC3339),
		}
	}

	if s.currentResult.Error != nil {
		response["error"] = s.currentResult.Error.Error()
	}

	if !s.currentResult.Valid {
		w.WriteHeader(http.StatusOK) // Still return 200 for status endpoint
	}

	json.NewEncoder(w).Encode(response)
}
