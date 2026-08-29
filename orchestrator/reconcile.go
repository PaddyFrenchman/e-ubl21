package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DispatchReconcileReport summarizes one durable StageRequest outbox scan.
type DispatchReconcileReport struct {
	Scanned    int
	Reconciled int
	Errors     int
}

// FinalResultReconcileReport summarizes one durable final-result outbox scan.
type FinalResultReconcileReport struct {
	Scanned    int
	Reconciled int
	Errors     int
}

// ReconcilePendingDispatches republishes every durable pending request and
// CAS-marks successful broker acknowledgements. Independent replicas may run
// this concurrently; deterministic message IDs and EnsureDispatch convergence
// prevent duplicate logical transitions.
func (machine *Machine) ReconcilePendingDispatches(ctx context.Context) (DispatchReconcileReport, error) {
	var report DispatchReconcileReport
	if err := machine.validate(); err != nil {
		return report, err
	}
	pending, err := machine.Queue.ValidationExecutionsRequiringDispatch(ctx)
	if err != nil {
		return report, err
	}
	errorsSeen := make([]error, 0)
	for _, execution := range pending {
		report.Scanned++
		if _, err := machine.EnsureDispatch(ctx, execution); err != nil {
			report.Errors++
			errorsSeen = append(errorsSeen, fmt.Errorf("reconcile validation %s generation %d: %w", execution.Execution.ValidationID, execution.Execution.Generation, err))
			continue
		}
		report.Reconciled++
	}
	return report, errors.Join(errorsSeen...)
}

// RunDispatchReconciler performs an immediate scan and then repeats until the
// context is canceled. observe is called for every cycle so transient failures
// are visible without stopping later repair attempts.
func (machine *Machine) RunDispatchReconciler(ctx context.Context, interval time.Duration, observe func(DispatchReconcileReport, error)) error {
	if interval <= 0 {
		return errors.New("orchestrator dispatch reconcile interval must be positive")
	}
	if observe == nil {
		return errors.New("orchestrator dispatch reconcile observer is required")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		report, err := machine.ReconcilePendingDispatches(ctx)
		observe(report, err)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// ReconcilePendingFinalResults publishes every terminal execution whose
// permanent result is absent or whose publication acknowledgement was not
// recorded. Results are rebuilt from immutable accepted stage evidence.
func (machine *Machine) ReconcilePendingFinalResults(ctx context.Context) (FinalResultReconcileReport, error) {
	var report FinalResultReconcileReport
	if err := machine.validate(); err != nil {
		return report, err
	}
	pending, err := machine.Queue.ValidationExecutionsRequiringFinalResult(ctx)
	if err != nil {
		return report, err
	}
	errorsSeen := make([]error, 0)
	for _, execution := range pending {
		report.Scanned++
		if _, err := machine.EnsureFinalResult(ctx, execution); err != nil {
			report.Errors++
			errorsSeen = append(errorsSeen, fmt.Errorf("reconcile final result for validation %s generation %d: %w", execution.Execution.ValidationID, execution.Execution.Generation, err))
			continue
		}
		report.Reconciled++
	}
	return report, errors.Join(errorsSeen...)
}

// RunFinalResultReconciler performs an immediate finalization scan and then
// repeats until cancellation. Transient failures remain visible to observe
// and are retried on later cycles.
func (machine *Machine) RunFinalResultReconciler(ctx context.Context, interval time.Duration, observe func(FinalResultReconcileReport, error)) error {
	if interval <= 0 {
		return errors.New("orchestrator final-result reconcile interval must be positive")
	}
	if observe == nil {
		return errors.New("orchestrator final-result reconcile observer is required")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		report, err := machine.ReconcilePendingFinalResults(ctx)
		observe(report, err)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
