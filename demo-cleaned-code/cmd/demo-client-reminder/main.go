package main

import (
	"context"
	"fmt"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/bootstrap"
)

func main() {
	ctx := context.Background()
	app, err := bootstrap.BuildServiceForDemo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	result, err := app.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		os.Exit(1)
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
