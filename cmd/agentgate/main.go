package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/kimzclandi/agentgate/internal/gate"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		log.Print(e)
		os.Exit(1)
	}
}
func run() error {
	tokenUser := flag.String("token", "", "issue local development token for seeded user")
	flag.Parse()
	dev := os.Getenv("AGENTGATE_DEV") == "1"
	root := os.Getenv("AGENTGATE_DATA")
	if root == "" {
		root = "data"
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	addr := os.Getenv("AGENTGATE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if dev {
		host, _, e := net.SplitHostPort(addr)
		if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("development mode must bind a loopback IP")
		}
	}
	// Token CLI must never open the DB: opening it performs restart recovery.
	a := &gate.Auth{}
	if dev {
		keyPath := filepath.Join(root, "dev.key")
		key, e := os.ReadFile(keyPath)
		if os.IsNotExist(e) {
			key = make([]byte, 32)
			if _, e = rand.Read(key); e != nil {
				return e
			}
			f, e := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return e
			}
			_, e = f.Write(key)
			f.Close()
			if e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if len(key) != 32 {
			return errors.New("invalid dev key")
		}
		a.DevKey = key
	}
	if *tokenUser != "" {
		if !dev {
			return errors.New("token command only available in explicit development mode")
		}
		t, e := a.Token(*tokenUser)
		if e != nil {
			return e
		}
		_, e = os.Stdout.WriteString(t + "\n")
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !dev {
		issuer, client := os.Getenv("OIDC_ISSUER"), os.Getenv("OIDC_CLIENT_ID")
		if issuer == "" || client == "" {
			return errors.New("set OIDC_ISSUER/OIDC_CLIENT_ID or explicitly set AGENTGATE_DEV=1")
		}
		init, c := context.WithTimeout(ctx, 10*time.Second)
		provider, e := oidc.NewProvider(init, issuer)
		c()
		if e != nil {
			return e
		}
		a.OIDC = provider.Verifier(&oidc.Config{ClientID: client})
	}
	// Exclusive process lock prevents a second server from interrupting active runs.
	lock, e := os.OpenFile(filepath.Join(root, "server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return errors.New("data directory already in use")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	store, e := gate.Open(filepath.Join(root, "agentgate.db"))
	if e != nil {
		return e
	}
	defer store.DB.Close()
	a.Store = store
	if dev {
		if e = store.Seed(ctx); e != nil {
			return e
		}
	}
	dirs := filepath.Join(root, "runs")
	if e = os.MkdirAll(dirs, 0700); e != nil {
		return e
	}
	engine := &gate.Engine{Store: store, Root: dirs}
	if e = engine.Sweep(ctx); e != nil {
		return e
	}
	handler := gate.NewServer(engine, a, http.FileServer(http.Dir("web")))
	if endpoint := os.Getenv("MODEL_ENDPOINT"); endpoint != "" {
		handler.Model, e = gate.NewModel(endpoint, os.Getenv("MODEL_API_KEY"), os.Getenv("MODEL_NAME"), os.Getenv("MODEL_ALLOWED_HOST"))
		if e != nil {
			return e
		}
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 6 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if e := engine.Sweep(ctx); e != nil && ctx.Err() == nil {
					log.Print("runtime cleanup failed")
				}
			}
		}
	}()
	errch := make(chan error, 1)
	go func() { log.Printf("AgentGate listening on %s (dev=%v)", addr, dev); errch <- server.ListenAndServe() }()
	select {
	case e = <-errch:
		stop()
	case <-ctx.Done():
	}
	stop()
	shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = server.Shutdown(shutdown)
	<-done
	if e != nil && !errors.Is(e, http.ErrServerClosed) {
		return e
	}
	return nil
}
