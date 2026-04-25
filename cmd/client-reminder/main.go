package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/bootstrap"
)

func main() {
	ctx := context.Background()
	app, err := bootstrap.BuildServiceFromEnv()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	result, err := app.Run(ctx)
	if err != nil {
		log.Fatalf("run failed: %v", err)
	}

	fmt.Fprintf(
		os.Stdout,
		"run complete: total=%d sent=%d skipped_done=%d missed_period_alerts=%d failures=%d\n",
		result.TotalCustomers,
		result.Sent,
		result.SkippedDone,
		result.MissedPeriodAlerts,
		result.Failures,
	)

	if result.Failures > 0 {
		os.Exit(1)
	}
}
