package kiro

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/user"
	"sync/atomic"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

// ProfileReader supplies one fresh, validated combined account snapshot.
type ProfileReader interface {
	ReadProfileSnapshot(context.Context, config.Session) (credentials.ProfileSnapshot, error)
}

// Adapter is a synchronous experimental generator. Concurrent calls fail busy.
// Its profile pin survives upstream failures and is never persisted.
type Adapter struct {
	reader    ProfileReader
	reference config.Session
	pin       [32]byte
	pinned    bool
	active    atomic.Bool
	disabled  atomic.Bool
	transport transport
	metadata  func() (string, error)
	// Private development hook; production always reads the unmodified body.
	responseReader func(io.Reader) io.Reader
}

// New validates frozen settings without opening the account source.
func New(reader ProfileReader, document config.Document) (*Adapter, error) {
	if reader == nil || document.Validate() != nil || document.Session == nil || document.Models[bridge.Model] != bridge.Model {
		return nil, errors.New("experimental bridge requires a linked session and exact claude-opus-5.5 mapping")
	}
	return &Adapter{reader: reader, reference: *document.Session, metadata: machineFingerprint}, nil
}

// Generate reads one snapshot, dispatches once, and validates and closes the
// upstream stream before exposing complete tool calls or inferred completion.
// A cleanup timeout disables inference while the cleanup owner retains its connection.
func (a *Adapter) Generate(parent context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	if a.disabled.Load() {
		return bridge.End{}, &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Inference cleanup failed. Restart the gateway.", Category: "cleanup_failed"}
	}
	if !a.active.CompareAndSwap(false, true) {
		return bridge.End{}, &bridge.Failure{Status: bridge.StatusOverloaded, Type: "overloaded_error", Message: "An inference request is already active.", Category: "busy"}
	}
	defer a.active.Store(false)
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return bridge.End{}, err
	}
	if r.Model != bridge.Model {
		return bridge.End{}, bridge.Invalid()
	}
	if err := bridge.Before(ctx, "source"); err != nil {
		return bridge.End{}, err
	}
	selected, err := a.reader.ReadProfileSnapshot(ctx, a.reference)
	if err != nil {
		return bridge.End{}, sourceFailure(err)
	}
	if err := ctx.Err(); err != nil {
		return bridge.End{}, err
	}
	digest := selected.ProfileDigest()
	if a.pinned && subtle.ConstantTimeCompare(a.pin[:], digest[:]) != 1 {
		return bridge.End{}, &bridge.Failure{Status: http.StatusConflict, Type: "api_error", Message: "Selected Kiro profile has changed. Check the selected profile through Kiro CLI, then restart the gateway.", Category: "profile_changed"}
	}
	region := selected.ProfileRegion()
	if region != "us-east-1" && region != "eu-central-1" {
		return bridge.End{}, sourceFailure(credentials.ErrProfileUnsupported)
	}
	a.pin, a.pinned = digest, true
	snapshot := selected.Credential()
	if !snapshot.ExpiresAt().After(time.Now()) {
		return bridge.End{}, sourceFailure(credentials.ErrExpired)
	}
	if !config.VisibleASCII(snapshot.AccessToken(), config.MaxBytes) {
		return bridge.End{}, sourceFailure(credentials.ErrRecord)
	}
	fingerprint, err := a.metadata()
	if err != nil {
		return bridge.End{}, localMetadataFailure()
	}
	invocation, err := newUUID()
	if err != nil {
		return bridge.End{}, localMetadataFailure()
	}
	conversation, err := newUUID()
	if err != nil {
		return bridge.End{}, localMetadataFailure()
	}
	body, err := encodeRequest(r, selected.ProfileARN(), conversation)
	if err != nil {
		return bridge.End{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://runtime."+region+".kiro.dev:443/generateAssistantResponse", io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return bridge.End{}, bridge.ProtocolFailure()
	}
	req.ContentLength = int64(len(body))
	req.Header = applicationHeaders(fingerprint, invocation)
	req.Header.Set("Authorization", "Bearer "+snapshot.AccessToken())
	state := streamState{request: r, emit: func(e bridge.Event) error {
		if e.Text != "" {
			bridge.Observe(ctx, "text", "success", false)
		}
		return emit(e)
	}}
	if err := ctx.Err(); err != nil {
		return bridge.End{}, err
	}
	if err := bridge.Before(ctx, "dispatch"); err != nil {
		return bridge.End{}, err
	}
	err = a.transport.exchange(ctx, req, func(resp *http.Response) error {
		if resp.StatusCode == http.StatusTooManyRequests {
			return &bridge.Failure{Status: http.StatusTooManyRequests, Type: "rate_limit_error", Message: "Upstream rate limit reached.", Category: "upstream_throttle"}
		}
		if resp.StatusCode != http.StatusOK {
			return &bridge.Failure{Status: http.StatusBadGateway, Type: "api_error", Message: "Upstream request failed.", Category: "upstream_status"}
		}
		media, params, e := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if e != nil || media != "application/vnd.amazon.eventstream" || len(params) != 0 || resp.Header.Get("Content-Encoding") != "" || resp.ContentLength > 8<<20 {
			return bridge.ProtocolFailure()
		}
		var input io.Reader = resp.Body
		if a.responseReader != nil {
			input = a.responseReader(input)
		}
		if err := readFrames(input, func(event string, b []byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return state.observe(event, b)
		}); err != nil {
			return err
		}
		return state.complete()
	})
	if err != nil {
		if bridge.SafeFailure(err).Category == "cleanup_failed" {
			a.disabled.Store(true)
		}
		return bridge.End{}, err
	}
	if err := ctx.Err(); err != nil {
		return bridge.End{}, err
	}
	if err := state.emitTools(); err != nil {
		return bridge.End{}, err
	}
	return bridge.End{Basis: bridge.InferredCleanEOF, ReasoningEvents: state.reasoning}, nil
}

func localMetadataFailure() error {
	return &bridge.Failure{Status: http.StatusInternalServerError, Type: "api_error", Message: "Required local request metadata is unavailable.", Category: "local_metadata"}
}
func machineFingerprint() (string, error) {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "", localMetadataFailure()
	}
	name := ""
	for _, key := range []string{"LOGNAME", "USER", "LNAME", "USERNAME"} {
		if name = os.Getenv(key); name != "" {
			break
		}
	}
	if name == "" {
		u, err := user.Current()
		if err != nil || u.Username == "" {
			return "", localMetadataFailure()
		}
		name = u.Username
	}
	sum := sha256.Sum256([]byte(host + "-" + name + "-kiro-gateway"))
	return hex.EncodeToString(sum[:]), nil
}
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func applicationHeaders(fingerprint, invocation string) http.Header {
	h := make(http.Header)
	for k, v := range map[string]string{
		"Content-Type": "application/x-amz-json-1.0", "X-Amz-Target": "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
		"User-Agent":       "aws-sdk-js/1.0.27 ua/2.1 os/win32#10.0.19044 lang/js md/nodejs#22.21.1 api/codewhispererstreaming#1.0.27 m/E KiroIDE-0.7.45-" + fingerprint,
		"X-Amz-User-Agent": "aws-sdk-js/1.0.27 KiroIDE-0.7.45-" + fingerprint, "X-Amzn-Codewhisperer-Optout": "true", "X-Amzn-Kiro-Agent-Mode": "vibe", "Amz-Sdk-Invocation-Id": invocation, "Amz-Sdk-Request": "attempt=1; max=3", "Accept": "*/*", "Accept-Encoding": "identity",
	} {
		h.Set(k, v)
	}
	return h
}
func sourceFailure(err error) *bridge.Failure {
	status, category, message := http.StatusServiceUnavailable, "source_unavailable", "Saved Kiro source is unavailable or unsupported. Check Kiro CLI sign in and local file access."
	for _, row := range []struct {
		err               error
		status            int
		category, message string
	}{
		{credentials.ErrRecord, http.StatusServiceUnavailable, "credential_invalid", "Saved Kiro token record is invalid or unsupported. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart."},
		{credentials.ErrExpired, http.StatusServiceUnavailable, "credential_expired", "Saved Kiro credential has expired. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart."},
		{credentials.ErrBusy, http.StatusServiceUnavailable, "source_busy", "Saved Kiro source is busy. Wait for Kiro CLI, then retry explicitly."},
		{credentials.ErrTimeout, http.StatusGatewayTimeout, "source_timeout", "Reading the saved Kiro source timed out. Check local source availability before retrying."},
		{credentials.ErrChanged, http.StatusConflict, "session_changed", "Saved Kiro session has changed. Stop the gateway, link again, restore the model mapping, and restart."},
		{credentials.ErrProfileInvalid, http.StatusServiceUnavailable, "profile_invalid", "Saved Kiro profile is invalid. Check the selected profile through Kiro CLI, then restart the gateway."},
		{credentials.ErrProfileUnsupported, http.StatusServiceUnavailable, "profile_unsupported", "Saved Kiro profile is unsupported by this gateway. Check the supported profile and region before restarting."},
		{credentials.ErrCanceled, http.StatusServiceUnavailable, "source_canceled", "Reading the saved Kiro source was canceled. Retry only when the gateway is ready."},
	} {
		if errors.Is(err, row.err) {
			status, category, message = row.status, row.category, row.message
			break
		}
	}
	return &bridge.Failure{Status: status, Type: "api_error", Message: message, Category: category}
}
