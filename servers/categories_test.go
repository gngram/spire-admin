package servers

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCategoryInitializationAndAccessors(t *testing.T) {
	sCtx := NewServerContext("test-server", "127.0.0.1", "8081", "")
	server := &SpireServer{
		ServerContext: sCtx,
	}
	server.initServices()

	if server.Agent() == nil {
		t.Fatal("expected AgentService to not be nil")
	}
	if server.Bundle() == nil {
		t.Fatal("expected BundleService to not be nil")
	}
	if server.Entry() == nil {
		t.Fatal("expected EntryService to not be nil")
	}
	if server.Federation() == nil {
		t.Fatal("expected FederationService to not be nil")
	}
	if server.HealthCheck() == nil {
		t.Fatal("expected HealthCheckService to not be nil")
	}
	if server.LocalAuthority() == nil {
		t.Fatal("expected LocalAuthorityService to not be nil")
	}
	if server.Logger() == nil {
		t.Fatal("expected LoggerService to not be nil")
	}
	if server.Token() == nil {
		t.Fatal("expected TokenService to not be nil")
	}
	if server.UpstreamAuthority() == nil {
		t.Fatal("expected UpstreamAuthorityService to not be nil")
	}
	if server.Validate() == nil {
		t.Fatal("expected ValidateService to not be nil")
	}
	if server.X509() == nil {
		t.Fatal("expected X509Service to not be nil")
	}
}

func TestCategoryPlaceholders(t *testing.T) {
	sCtx := NewServerContext("test-server", "127.0.0.1", "8081", "")
	server := &SpireServer{
		ServerContext: sCtx,
	}
	server.initServices()
	ctx := context.Background()

	// Logger placeholders
	if _, err := server.Logger().GetLogLevel(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for GetLogLevel, got %v", err)
	}
	if err := server.Logger().SetLogLevel(ctx, "DEBUG"); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for SetLogLevel, got %v", err)
	}
	if err := server.Logger().ResetLogLevel(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ResetLogLevel, got %v", err)
	}

	// Validate placeholders
	if err := server.Validate().ValidateConfig(ctx, []byte("")); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ValidateConfig, got %v", err)
	}
	if err := server.Validate().ValidateBundle(ctx, []byte("")); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ValidateBundle, got %v", err)
	}

	// Token placeholder
	if _, err := server.Token().ValidateToken(ctx, "sample-token"); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ValidateToken, got %v", err)
	}

	// X509 placeholders
	if _, err := server.X509().MintX509SVID(ctx, "spiffe://example.org/workload", time.Hour); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for MintX509SVID, got %v", err)
	}
	if _, err := server.X509().ValidateX509SVID(ctx, []byte("")); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ValidateX509SVID, got %v", err)
	}

	// Upstream Authority placeholder
	if _, err := server.UpstreamAuthority().ShowUpstreamAuthority(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ShowUpstreamAuthority, got %v", err)
	}

	// Local Authority placeholder
	if _, err := server.LocalAuthority().ShowLocalJWTAuthorities(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for ShowLocalJWTAuthorities, got %v", err)
	}

	// Agent placeholder
	if _, err := server.Agent().CountAgents(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for CountAgents, got %v", err)
	}

	// Bundle placeholder
	if _, err := server.Bundle().CountBundles(ctx); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for CountBundles, got %v", err)
	}
}

func TestParseSPIFFEID(t *testing.T) {
	// Valid SPIFFE ID
	validID := "spiffe://domain.test/ns/default/sa/test"
	parsed, err := ParseSPIFFEID(validID)
	if err != nil {
		t.Fatalf("unexpected error parsing valid SPIFFE ID: %v", err)
	}
	if parsed.TrustDomain != "domain.test" {
		t.Errorf("expected trust domain 'domain.test', got '%s'", parsed.TrustDomain)
	}
	if parsed.Path != "/ns/default/sa/test" {
		t.Errorf("expected path '/ns/default/sa/test', got '%s'", parsed.Path)
	}

	// Method on X509Service
	x509Svc := NewX509Service(nil)
	parsedViaSvc, err := x509Svc.ParseSPIFFEID(validID)
	if err != nil {
		t.Fatalf("unexpected error parsing via X509Service: %v", err)
	}
	if parsedViaSvc.TrustDomain != "domain.test" {
		t.Errorf("expected trust domain 'domain.test', got '%s'", parsedViaSvc.TrustDomain)
	}

	// Invalid SPIFFE ID
	_, err = ParseSPIFFEID("invalid-prefix://domain.test/test")
	if err == nil {
		t.Error("expected error for invalid SPIFFE ID prefix, got nil")
	}
}

func TestServerValidationInputs(t *testing.T) {
	// Invalid IP address
	if _, err := NewSpireServer("srv", "invalid-ip", "8081", ""); err == nil {
		t.Error("expected error for invalid IP address, got nil")
	}

	// Invalid Port
	if _, err := NewSpireServer("srv", "127.0.0.1", "99999", ""); err == nil {
		t.Error("expected error for port > 65535, got nil")
	}
	if _, err := NewSpireServer("srv", "127.0.0.1", "abc", ""); err == nil {
		t.Error("expected error for non-integer port, got nil")
	}
}
