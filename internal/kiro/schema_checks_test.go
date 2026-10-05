//go:build schemaprobe

package kiro

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

const schemaFixture = `{"models":[{"modelId":"sentinel-other-model","additionalModelRequestFieldsSchema":{"description":"sentinel-secret"}},{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{"properties":{"system_prompt":{"type":"string","description":"sentinel-description"},"max_tokens":{"type":"integer","minimum":0,"maximum":2147483647},"output_config":{"properties":{"effort":{"type":"string","enum":["high","low","high","sentinel-enum",1],"default":"high"}}},"thinking":{"$ref":"sentinel-reference","properties":{"type":{"type":"string"}}}}}}],"nextToken":"sentinel-page","requestId":"sentinel-request"}`

func schemaFixtureDependencies(t *testing.T, home string) schemaDependencies {
	t.Helper()
	return schemaDependencies{
		preflight: func(context.Context) (string, string, error) {
			return strings.Repeat("a", 40), strings.Repeat("b", 64), nil
		},
		home:   func() (string, error) { return home, nil },
		open:   func(home string) (schemaStore, error) { return configstore.OpenExisting(home) },
		reader: func(home string) ProfileReader { return credentials.Reader{Home: home} },
		wire: transport{dial: func(context.Context, string) (net.Conn, error) {
			t.Error("schema fixture dial reached, want predispatch rejection")
			return nil, errPlanInvalid
		}},
	}
}

func schemaCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"management.us-east-1.kiro.dev", "management.eu-central-1.kiro.dev"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

func schemaTLSFixture(t *testing.T, handler http.HandlerFunc) transport {
	t.Helper()
	cert, roots := schemaCertificate(t)
	s := httptest.NewUnstartedServer(handler)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	s.StartTLS()
	t.Cleanup(s.Close)
	return transport{roots: roots, dial: func(ctx context.Context, address string) (net.Conn, error) {
		if address != "management.us-east-1.kiro.dev:443" && address != "management.eu-central-1.kiro.dev:443" {
			return nil, errPlanInvalid
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp4", s.Listener.Addr().String())
	}}
}

// covers: AC-16. Real settings and SQLite feed a single authenticated TLS
// catalogue request. Only structural values survive into the report.
func TestSchemaProbeSyntheticPath(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-1"} {
		t.Run(region, func(t *testing.T) {
			home, db := probeHome(t)
			profile := strings.Replace(probeSyntheticProfile, "us-east-1", region, 1)
			if _, err := db.Exec(`UPDATE state SET value=?`, profile); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(home, ".config", "kiro-gateway", "config.json")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			d := schemaFixtureDependencies(t, home)
			reads, requests := 0, atomic.Int32{}
			d.reader = func(home string) ProfileReader {
				return wireSnapshotFunc(func(ctx context.Context, ref config.Session) (credentials.ProfileSnapshot, error) {
					reads++
					return (credentials.Reader{Home: home}).ReadProfileSnapshot(ctx, ref)
				})
			}
			d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != "POST" || r.URL.RequestURI() != "/" || r.Host != "management."+region+".kiro.dev:443" || !r.Close || r.Proto != "HTTP/1.1" {
					t.Error("schema request route or lifecycle differs, want fixed POST and connection close")
				}
				wantHeaders := http.Header{}
				for k, v := range map[string]string{"Authorization": "Bearer sentinel-token", "X-Amz-Target": schemaTarget, "Content-Type": "application/x-amz-json-1.0", "Accept": "application/json", "Accept-Encoding": "identity", "User-Agent": "kiro-gateway-schema-probe/1", "X-Amzn-Codewhisperer-Optout": "true", "Connection": "close"} {
					wantHeaders.Set(k, v)
				}
				wantHeaders.Set("Content-Length", r.Header.Get("Content-Length"))
				if !reflect.DeepEqual(r.Header, wantHeaders) {
					t.Error("schema request headers differ, want exact finite headers")
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || !reflect.DeepEqual(body, map[string]any{"origin": "KIRO_CLI", "maxResults": float64(100), "profileArn": "arn:aws:codewhisperer:" + region + ":000000000000:profile/sentinel-profile"}) {
					t.Error("schema request body differs, want exact snapshot ARN and two constants")
				}
				if lock, err := configstore.OpenExisting(home); !errors.Is(err, configstore.ErrLocked) {
					if lock != nil {
						lock.Close()
					}
					t.Error("schema request lock absent, want held through response cleanup")
				}
				w.Header().Set("Content-Type", "application/x-amz-json-1.0; charset=UTF-8")
				w.Header().Set("Content-Encoding", "identity")
				_, _ = io.WriteString(w, schemaFixture)
			})
			r := runSchemaProbe(t.Context(), true, d)
			if r.Outcome != "schema_observed" || r.FailureCategory != nil || r.CleanupOutcome != "complete" || r.DispatchCount != 1 || reads != 1 || requests.Load() != 1 || r.Region == nil || *r.Region != region || r.MorePages == nil || !*r.MorePages || *r.CatalogueComplete {
				t.Errorf("runSchemaProbe(synthetic %s) = %+v reads=%d requests=%d, want observed once with incomplete page", region, r, reads, requests.Load())
			}
			b, err := encodeSchemaReport(r)
			if err != nil || len(b) > 16<<10 || bytes.Contains(b, []byte("sentinel")) || bytes.Contains(b, []byte("arn:aws")) || bytes.Contains(b, []byte("sha256:")) {
				t.Error("encodeSchemaReport(synthetic) retained forbidden data or exceeded its bound")
			}
			if r.Fields[1].Type == nil || *r.Fields[1].Type != "string" || !reflect.DeepEqual(r.Fields[4].EnumValues, []string{"high", "low"}) || r.Fields[6].Present != nil {
				t.Errorf("extractSchema(synthetic).fields = %+v, want filtered finite values and unknown reference", r.Fields)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Error("runSchemaProbe(synthetic) changed settings, want exact original bytes")
			}
			lock, err := configstore.OpenExisting(home)
			if err != nil {
				t.Fatalf("OpenExisting(after probe) = %v, want released lock", err)
			}
			lock.Close()
		})
	}
}

// covers: AC-16. Only an inspected missing property establishes absence.
func TestSchemaPathUnknownAndAbsent(t *testing.T) {
	for _, tc := range []struct {
		name, schema, path, shape string
		present                   *bool
	}{
		{"root missing", `{}`, "system", "other", nil},
		{"root properties null", `{"properties":null}`, "system", "other", nil},
		{"root properties array", `{"properties":[]}`, "system", "other", nil},
		{"root reference", `{"$ref":null,"properties":{}}`, "system", "other", nil},
		{"explicit absence", `{"properties":{}}`, "system", "absent", schemaPtr(false)},
		{"intermediate absent", `{"properties":{}}`, "output_config.effort", "absent", schemaPtr(false)},
		{"intermediate missing map", `{"properties":{"output_config":{}}}`, "output_config.effort", "other", nil},
		{"intermediate null map", `{"properties":{"output_config":{"properties":null}}}`, "output_config.effort", "other", nil},
		{"intermediate array map", `{"properties":{"output_config":{"properties":[]}}}`, "output_config.effort", "other", nil},
		{"final absent", `{"properties":{"output_config":{"properties":{}}}}`, "output_config.effort", "absent", schemaPtr(false)},
		{"intermediate boolean", `{"properties":{"output_config":true}}`, "output_config.effort", "other", nil},
		{"intermediate ref", `{"properties":{"output_config":{"$ref":"sentinel","properties":{"effort":{}}}}}`, "output_config.effort", "other", nil},
		{"final ref", `{"properties":{"system":{"$ref":false,"type":"string"}}}`, "system", "other", nil},
		{"final boolean", `{"properties":{"system":false}}`, "system", "other", schemaPtr(true)},
		{"final null", `{"properties":{"system":null}}`, "system", "other", schemaPtr(true)},
		{"final object", `{"properties":{"system":{}}}`, "system", "object", schemaPtr(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := schemaJSON([]byte(tc.schema), schemaBodyLimit)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := inspectSchemaPath(root, tc.path)
			if !ok || got.Shape != tc.shape || !reflect.DeepEqual(got.Present, tc.present) {
				t.Errorf("inspectSchemaPath(%s,%s) = %+v, want %s present=%v", tc.name, tc.path, got, tc.shape, tc.present)
			}
			if tc.shape != "object" {
				want := schemaField{Path: tc.path, Shape: tc.shape, Present: tc.present}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("inspectSchemaPath(%s) = %+v, want %+v with all derived fields null", tc.name, got, want)
				}
			}
		})
	}
}

// covers: AC-16. An object schema may be observed while every fixed path is
// unknown. Success still requires the request and resource cleanup to finish.
func TestSchemaProbeObservesUnknownPaths(t *testing.T) {
	for _, schema := range []string{`{}`, `{"$ref":"sentinel-reference","properties":{"system":{"type":"string"}}}`} {
		t.Run(schema, func(t *testing.T) {
			d, store := schemaMemoryDependencies(t)
			d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":`+schema+`}]}`)
			})
			r := runSchemaProbe(t.Context(), true, d)
			if r.Outcome != "schema_observed" || r.FailureCategory != nil || r.CleanupOutcome != "complete" || !store.closed || r.DispatchCount != 1 {
				t.Errorf("runSchemaProbe(%s) = %+v, closed=%t, want observed once after complete cleanup", schema, r, store.closed)
			}
			if r.ModelFound == nil || !*r.ModelFound || r.SchemaPresent == nil || !*r.SchemaPresent || r.CatalogueComplete == nil || !*r.CatalogueComplete || r.MorePages == nil || *r.MorePages {
				t.Errorf("runSchemaProbe(%s) = %+v, want one selected object on a complete page", schema, r)
			}
			want := []schemaField{
				{Path: "system", Shape: "other"}, {Path: "system_prompt", Shape: "other"},
				{Path: "messages", Shape: "other"}, {Path: "max_tokens", Shape: "other"},
				{Path: "output_config.effort", Shape: "other"}, {Path: "reasoning.effort", Shape: "other"},
				{Path: "thinking.type", Shape: "other"}, {Path: "thinking.display", Shape: "other"},
			}
			if !reflect.DeepEqual(r.Fields, want) {
				t.Errorf("runSchemaProbe(%s).Fields = %+v, want %+v", schema, r.Fields, want)
			}
		})
	}
}

// covers: AC-16. A later unsupported field must not publish a partial schema.
func TestSchemaExtractionDiscardsPartialFields(t *testing.T) {
	body := `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":{"properties":{"system":{"type":"string"},"thinking":{"properties":{"display":{"enum":[` + strings.TrimSuffix(strings.Repeat(`"high",`, 33), ",") + `]}}}}}}]}`
	r := newSchemaReport()
	wantFields := append([]schemaField(nil), r.Fields...)
	extractSchema([]byte(body), &r)
	if r.FailureCategory == nil || *r.FailureCategory != "schema_unsupported" || r.Outcome != "needs_evidence" {
		t.Errorf("extractSchema(late enum overflow) = %+v, want schema_unsupported and needs_evidence", r)
	}
	if !reflect.DeepEqual(r.Fields, wantFields) {
		t.Errorf("extractSchema(late enum overflow).Fields = %+v, want %+v without partial observations", r.Fields, wantFields)
	}
}

func TestSchemaCatalogueFailures(t *testing.T) {
	for _, tc := range []struct{ name, body, failure string }{
		{"missing models", `{}`, "invalid_response"},
		{"null models", `{"models":null}`, "invalid_response"},
		{"model not object", `{"models":[false]}`, "invalid_response"},
		{"missing model", `{"models":[]}`, "model_missing"},
		{"missing paginated", `{"models":[],"nextToken":"secret"}`, "model_missing"},
		{"display name", `{"models":[{"modelName":"claude-opus-5.5"}],"defaultModel":"claude-opus-5.5"}`, "model_missing"},
		{"duplicate model", `{"models":[{"modelId":"claude-opus-5.5"},{"modelId":"claude-opus-5.5"}]}`, "invalid_response"},
		{"missing schema", `{"models":[{"modelId":"claude-opus-5.5"}]}`, "schema_missing"},
		{"null schema", `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":null}]}`, "schema_missing"},
		{"string schema", `{"models":[{"modelId":"claude-opus-5.5","additionalModelRequestFieldsSchema":"sentinel"}]}`, "schema_unsupported"},
		{"empty pagination", `{"models":[],"nextToken":""}`, "invalid_response"},
		{"wrong pagination", `{"models":[],"nextToken":1}`, "invalid_response"},
		{"duplicate discarded key", `{"models":[{"modelId":"other","nested":{"x":1,"x":2}}]}`, "invalid_response"},
		{"duplicate escaped key", `{"models":[],"\u006dodels":[]}`, "invalid_response"},
		{"trailing", `{"models":[]} {}`, "invalid_response"},
		{"UTF8", "{\"models\":[],\"x\":\"\xff\"}", "invalid_response"},
		{"too many models", `{"models":[` + strings.TrimSuffix(strings.Repeat(`{},`, 101), ",") + `]}`, "invalid_response"},
		{"too deep discarded", `{"models":[],"x":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`, "invalid_response"},
		{"too large", strings.Repeat(" ", schemaBodyLimit+1), "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSchemaReport()
			extractSchema([]byte(tc.body), &r)
			if r.FailureCategory == nil || *r.FailureCategory != tc.failure || r.Outcome != "needs_evidence" {
				t.Errorf("extractSchema(%s) = %+v, want %s and needs_evidence", tc.name, r, tc.failure)
			}
			b, err := encodeSchemaReport(r)
			if err != nil || bytes.Contains(b, []byte("sentinel")) || bytes.Contains(b, []byte("secret")) {
				t.Error("extractSchema(failure) leaked discarded content")
			}
		})
	}
}

func TestSchemaObjectFields(t *testing.T) {
	for _, tc := range []struct {
		name, node string
		want       schemaField
		valid      bool
	}{
		{"unknown type and enum", `{"type":["string","null"],"enum":null,"default":null,"minimum":-1,"maximum":2147483648}`, schemaField{Type: schemaPtr("unknown"), EnumState: schemaPtr("other"), EnumValues: []string{}, OtherDefaultValue: schemaPtr(true)}, true},
		{"empty object", `{}`, schemaField{Type: schemaPtr("unknown"), EnumState: schemaPtr("absent"), EnumValues: []string{}, OtherDefaultValue: schemaPtr(false)}, true},
		{"finite filter", `{"type":"integer","minimum":0,"maximum":2147483647,"enum":["max","minimal","none","xhigh","disabled","enabled","adaptive","summarized","omitted","low","medium","high","low",null,"sentinel"],"default":"enabled"}`, schemaField{Type: schemaPtr("integer"), Minimum: schemaPtr(int64(0)), Maximum: schemaPtr(int64(2147483647)), EnumState: schemaPtr("array"), EnumValues: []string{"adaptive", "disabled", "enabled", "high", "low", "max", "medium", "minimal", "none", "omitted", "summarized", "xhigh"}, OtherEnumValues: schemaPtr(true), DefaultValue: schemaPtr("enabled"), OtherDefaultValue: schemaPtr(false)}, true},
		{"enum 32", `{"enum":[` + strings.TrimSuffix(strings.Repeat(`"high",`, 32), ",") + `]}`, schemaField{Type: schemaPtr("unknown"), EnumState: schemaPtr("array"), EnumValues: []string{"high"}, OtherEnumValues: schemaPtr(false), OtherDefaultValue: schemaPtr(false)}, true},
		{"enum 33", `{"enum":[` + strings.TrimSuffix(strings.Repeat(`"high",`, 33), ",") + `]}`, schemaField{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := schemaJSON([]byte(`{"properties":{"system":`+tc.node+`}}`), schemaBodyLimit)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := inspectSchemaPath(root, "system")
			if ok != tc.valid {
				t.Errorf("inspectSchemaPath(%s) valid=%t, want %t", tc.name, ok, tc.valid)
			}
			if ok {
				tc.want.Path = "system"
				tc.want.Present = schemaPtr(true)
				tc.want.Shape = "object"
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("inspectSchemaPath(%s) = %+v, want %+v", tc.name, got, tc.want)
				}
			}
		})
	}
}
