package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	githubpkg "github.com/iamhectordev/hector/integrations/github"
	"github.com/iamhectordev/hector/internal/tracing"
	pkgtools "github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestRuntimeSupervisorOwnsTracingShutdownOnNormalStop(t *testing.T) {
	tracingRuntime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: filepath.Join(t.TempDir(), "traces.jsonl"),
		},
	})
	require.NoError(t, err)

	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	started := make(chan struct{}, 1)
	runtime := &Runtime{
		cfg:     &Config{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		tracing: tracingRuntime,
		bus:     bus,
	}
	require.NoError(t, runtime.initSupervisor(t.Context(), []supervisor.Module{
		blockingModule{started: started},
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan supervisor.Report, 1)
	go func() {
		done <- runtime.sv.Run(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("module did not start")
	}

	cancel()
	var rep supervisor.Report
	select {
	case rep = <-done:
	case <-time.After(time.Second):
		t.Fatal("supervisor did not stop")
	}

	require.Equal(t, supervisor.ReasonContextCanceled, rep.Reason)
	require.Empty(t, rep.PostStopErrors)
	require.Nil(t, runtime.tracing)
	runtime.close(ctx)
}

func TestInitIntegrationsRegistersGitHubToolsWhenEnabled(t *testing.T) {
	t.Parallel()

	privateKeyPath := writeTestPrivateKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/456/access_tokens":
			w.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(w, `{"token":"installation-token","expires_at":"2026-05-22T12:00:00Z"}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	runtime := &Runtime{
		cfg: &Config{
			GitHub: githubpkg.Config{
				Enabled:        true,
				AppID:          123,
				InstallationID: 456,
				PrivateKeyPath: privateKeyPath,
				APIURL:         server.URL,
			},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	integs, err := runtime.buildIntegrations(t.Context())
	require.NoError(t, err)
	require.Len(t, integs, 1)

	registry, err := pkgtools.NewRegistry()
	require.NoError(t, err)

	modules, err := runtime.initIntegrations(integs, registry)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	require.Equal(t, "integration.github", modules[0].Name())

	defs := registry.Definitions()
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	require.Contains(t, names, "get_issue")
	require.Contains(t, names, "create_milestone")
	require.Contains(t, names, "list_milestones")
	require.Contains(t, names, "update_milestone")
	require.Contains(t, names, "create_blocked_by_relationship")
	require.Contains(t, names, "remove_blocked_by_relationship")
}

func TestInitIntegrationsSkipsGitHubWhenDisabled(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{
		cfg: &Config{
			GitHub: githubpkg.Config{Enabled: false},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	integs, err := runtime.buildIntegrations(t.Context())
	require.NoError(t, err)
	require.Empty(t, integs)

	registry, err := pkgtools.NewRegistry()
	require.NoError(t, err)

	modules, err := runtime.initIntegrations(integs, registry)
	require.NoError(t, err)
	require.Empty(t, modules)
	require.Empty(t, registry.Definitions())
}

type blockingModule struct {
	started chan<- struct{}
}

func (m blockingModule) Name() string { return "block" }

func (m blockingModule) Init(context.Context) error { return nil }

func (m blockingModule) Start(ctx context.Context) error {
	m.started <- struct{}{}
	<-ctx.Done()
	return nil
}

func (m blockingModule) Stop(context.Context) error { return nil }

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "github-app.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
	return path
}
