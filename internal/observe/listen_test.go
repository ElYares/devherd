package observe

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func newTestServer(t *testing.T) Server {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "observe.db")
	manager := NewManager(dbPath)
	if _, err := manager.Ensure(ctx); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	db, err := manager.Open()
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewServerWithDocker(NewStore(db), dbPath, nil)
}

func freeLoopbackPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split reserved addr: %v", err)
	}

	return port
}

func waitForHealth(t *testing.T, addr string) bool {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	for i := 0; i < 40; i++ {
		resp, err := client.Get("http://" + addr + "/health")
		if err == nil {
			resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}
		time.Sleep(25 * time.Millisecond)
	}

	return false
}

func TestListenAndServeOnServesEveryAddress(t *testing.T) {
	// 127.0.0.2 hace de segunda direccion del host: es el equivalente local al
	// gateway de infra_web, que es lo que ven los contenedores.
	port := freeLoopbackPort(t)
	primary := net.JoinHostPort("127.0.0.1", port)
	secondary := net.JoinHostPort("127.0.0.2", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		errc <- newTestServer(t).ListenAndServeOn(ctx, primary, secondary)
	}()

	if !waitForHealth(t, primary) {
		t.Fatalf("collector is not healthy at %s", primary)
	}
	if !waitForHealth(t, secondary) {
		t.Fatalf("collector is not healthy at %s", secondary)
	}

	cancel()
	select {
	case <-errc:
	case <-time.After(10 * time.Second):
		t.Fatal("ListenAndServeOn did not return after cancel")
	}
}

func TestListenAndServeOnToleratesAnUnavailableExtraAddress(t *testing.T) {
	// El gateway puede no existir todavia; eso no debe impedir que el collector
	// arranque en loopback.
	port := freeLoopbackPort(t)
	primary := net.JoinHostPort("127.0.0.1", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		errc <- newTestServer(t).ListenAndServeOn(ctx, primary, "192.0.2.1:9777")
	}()

	if !waitForHealth(t, primary) {
		t.Fatalf("collector is not healthy at %s", primary)
	}

	cancel()
	select {
	case <-errc:
	case <-time.After(10 * time.Second):
		t.Fatal("ListenAndServeOn did not return after cancel")
	}
}

func TestListenAndServeOnFailsWhenThePrimaryAddressIsTaken(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer listener.Close()

	if err := newTestServer(t).ListenAndServeOn(context.Background(), listener.Addr().String()); err == nil {
		t.Fatal("expected an error when the primary address is already bound")
	}
}
