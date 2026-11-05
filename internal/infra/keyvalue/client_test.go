package keyvalue

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/infra/keyvalue/kvtest"
	"github.com/redis/go-redis/v9"
)

func TestConnect_URLInvalida(t *testing.T) {
	_, err := Connect(context.Background(), "http://localhost:6379")

	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
}

func TestConnect_InalcancavelRetornaErroDePing(t *testing.T) {
	start := time.Now()

	_, err := Connect(context.Background(), "redis://127.0.0.1:1/0")

	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("Connect demorou %v, teto é o ping de 3s", elapsed)
	}
}

func TestConnect_AplicaTimeoutsEPool(t *testing.T) {
	options, err := optionsFromURL("redis://localhost:6379/0")
	if err != nil {
		t.Fatalf("optionsFromURL erro: %v", err)
	}

	if options.DialTimeout != 2*time.Second || options.ReadTimeout != 500*time.Millisecond ||
		options.WriteTimeout != 500*time.Millisecond || options.PoolTimeout != time.Second {
		t.Errorf("timeouts inesperados: %+v", options)
	}
	if options.PoolSize != 5 || options.MaxRetries != 1 {
		t.Errorf("PoolSize=%d MaxRetries=%d, want 5 e 1", options.PoolSize, options.MaxRetries)
	}
	if options.MaxActiveConns != 5 || options.PipelinePoolSize != 2 {
		t.Errorf("MaxActiveConns=%d PipelinePoolSize=%d, want 5 e 2 (teto de 7 conexões)", options.MaxActiveConns, options.PipelinePoolSize)
	}
}

func TestConnect_AceitaRediss(t *testing.T) {
	options, err := optionsFromURL("rediss://localhost:6379/0")
	if err != nil {
		t.Fatalf("optionsFromURL erro: %v", err)
	}

	if options.TLSConfig == nil {
		t.Error("rediss:// deveria habilitar TLS")
	}
}

func TestConnect_ServidorReal(t *testing.T) {
	client, err := Connect(context.Background(), kvtest.URL(t))
	if err != nil {
		t.Fatalf("Connect erro: %v", err)
	}
	defer client.Close()

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Errorf("Ping erro: %v", err)
	}
}

func TestParseServerVersion(t *testing.T) {
	cases := map[string]string{
		"valkey":  "# Server\r\nredis_version:7.2.4\r\nvalkey_version:8.1.10\r\nos:Linux\r\n",
		"redis":   "# Server\r\nredis_version:7.4.1\r\nos:Linux\r\n",
		"ausente": "# Server\r\nos:Linux\r\n",
	}
	want := map[string]string{"valkey": "8.1.10", "redis": "7.4.1", "ausente": "unknown"}

	for name, info := range cases {
		if got := parseServerVersion(info); got != want[name] {
			t.Errorf("%s: parseServerVersion() = %q, want %q", name, got, want[name])
		}
	}
}

func TestNewClient_NaoConectaAoCriar(t *testing.T) {
	client, err := NewClient("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatalf("NewClient erro: %v", err)
	}
	defer client.Close()

	if _, err := ServerVersion(context.Background(), client); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestServerVersion_ServidorReal(t *testing.T) {
	client, err := Connect(context.Background(), kvtest.URL(t))
	if err != nil {
		t.Fatalf("Connect erro: %v", err)
	}
	defer client.Close()

	version, err := ServerVersion(context.Background(), client)

	if err != nil || version == "" || version == "unknown" {
		t.Errorf("version = %q, err = %v, want versão real", version, err)
	}
}

func connectedClients(t *testing.T, client *redis.Client) int {
	t.Helper()
	info, err := client.Info(context.Background(), "clients").Result()
	if err != nil {
		t.Fatalf("INFO erro: %v", err)
	}
	for _, line := range strings.Split(info, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "connected_clients:"); ok {
			count, _ := strconv.Atoi(value)
			return count
		}
	}
	t.Fatal("connected_clients ausente do INFO")
	return 0
}

func TestConnect_UmaInstanciaUsaNoMaximoSeteConexoes(t *testing.T) {
	url := kvtest.URL(t)
	monitor, err := NewClient(url)
	if err != nil {
		t.Fatalf("NewClient erro: %v", err)
	}
	defer monitor.Close()
	baseline := connectedClients(t, monitor)

	client, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("Connect erro: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	for range 60 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pipe := client.Pipeline()
			pipe.Set(context.Background(), "vz:test-conn-ceiling", "1", time.Second)
			pipe.Get(context.Background(), "vz:test-conn-ceiling")
			_, _ = pipe.Exec(context.Background())
			client.Ping(context.Background())
		}()
	}
	wg.Wait()

	if used := connectedClients(t, monitor) - baseline; used > 7 {
		t.Errorf("conexões da instância = %d, want no máximo 7", used)
	}
}
