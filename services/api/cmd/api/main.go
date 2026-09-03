// Command api runs the RAZE Go control plane.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/aksayush2005/raze/services/api/internal/ai"
	"github.com/aksayush2005/raze/services/api/internal/engine"
	"github.com/aksayush2005/raze/services/api/internal/handlers"
	"github.com/aksayush2005/raze/services/api/internal/razorpay"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
	"github.com/aksayush2005/raze/services/api/internal/services"
)

func main() {
	_ = godotenv.Load() // best-effort; already-set env vars win

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	workers := 4
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	store := repositories.New(pool)
	eng := engine.New(store)
	// A typed-nil *ai.Client must not be wrapped in the Investigator interface:
	// an interface holding a nil pointer is non-nil, which would make the
	// reconciler call a nil client and crash the worker. Pass a nil interface
	// when the AI service is disabled.
	var investigator services.Investigator
	if u := os.Getenv("AI_SERVICE_URL"); u != "" {
		investigator = ai.NewClient(u)
	}
	recon := services.NewReconciler(store, eng, investigator, workers)
	review := services.NewReviewService(store)

	// Razorpay TEST MODE only. Never instantiate unless mode is explicitly test
	// and credentials are present.
	var rp *razorpay.Client
	if os.Getenv("RAZORPAY_MODE") == "test" &&
		os.Getenv("RAZORPAY_KEY_ID") != "" && os.Getenv("RAZORPAY_KEY_SECRET") != "" {
		rp = razorpay.New(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
	} else {
		log.Printf("razorpay: disabled (test-mode credentials absent); synthetic data only")
	}

	app := handlers.NewApp(store, recon, review, rp)

	// Async reconciliation worker (Phase 7).
	go recon.RunWorker(ctx)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: app.Routes(),
	}
	go func() {
		log.Printf("api listening on :%s (workers=%d, ai=%v)", port, workers, investigator != nil)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("api stopped")
}
