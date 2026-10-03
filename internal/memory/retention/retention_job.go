package retention

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RetentionPolicy defines how long each data type should be retained per PRD §8.3.
var RetentionPolicy = struct {
	ChatMessage      time.Duration
	RollingSummary   time.Duration
	InteractionEvent time.Duration
	AuditLog         time.Duration
}{
	ChatMessage:      30 * 24 * time.Hour,
	RollingSummary:   180 * 24 * time.Hour,
	InteractionEvent: 90 * 24 * time.Hour,
	AuditLog:         365 * 24 * time.Hour,
}

// Job performs daily deletion of expired data across all memory tables.
type Job struct {
	dbConnection *gorm.DB
}

// NewJob creates a new retention Job.
func NewJob(dbConnection *gorm.DB) *Job {
	return &Job{dbConnection: dbConnection}
}

// Run executes the retention cleanup for all memory tables.
// Intended to be called by the scheduler at 03:30 WIB daily ("30 3 * * *").
func (retentionJob *Job) Run(ctx context.Context) {
	logrus.Info("Retention job: starting daily memory cleanup")

	retentionJob.deleteExpiredChatMessages(ctx)
	retentionJob.deleteExpiredInteractionEvents(ctx)
	retentionJob.deleteExpiredAuditLogs(ctx)
	retentionJob.pruneExpiredSessionSummaries(ctx)
	retentionJob.markStaleInferredMemoryAsPending(ctx)

	logrus.Info("Retention job: completed")
}

// deleteExpiredChatMessages hard-deletes raw chat messages older than 30 days.
func (retentionJob *Job) deleteExpiredChatMessages(ctx context.Context) {
	cutoff := time.Now().Add(-RetentionPolicy.ChatMessage)
	result := retentionJob.dbConnection.WithContext(ctx).
		Exec("DELETE FROM chat_message WHERE created_at < ?", cutoff)
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("retention: failed to delete expired chat messages")
		return
	}
	logrus.WithField("rows_deleted", result.RowsAffected).Info("retention: deleted expired chat messages")
}

// deleteExpiredInteractionEvents hard-deletes interaction events older than 90 days.
func (retentionJob *Job) deleteExpiredInteractionEvents(ctx context.Context) {
	cutoff := time.Now().Add(-RetentionPolicy.InteractionEvent)
	result := retentionJob.dbConnection.WithContext(ctx).
		Exec("DELETE FROM user_interaction_event WHERE created_at < ?", cutoff)
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("retention: failed to delete expired interaction events")
		return
	}
	logrus.WithField("rows_deleted", result.RowsAffected).Info("retention: deleted expired interaction events")
}

// deleteExpiredAuditLogs hard-deletes audit log entries older than 12 months.
// Sensitive content is never stored in audit logs so this is safe.
func (retentionJob *Job) deleteExpiredAuditLogs(ctx context.Context) {
	cutoff := time.Now().Add(-RetentionPolicy.AuditLog)
	result := retentionJob.dbConnection.WithContext(ctx).
		Exec("DELETE FROM memory_audit_log WHERE created_at < ?", cutoff)
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("retention: failed to delete expired audit logs")
		return
	}
	logrus.WithField("rows_deleted", result.RowsAffected).Info("retention: deleted expired audit logs")
}

// pruneExpiredSessionSummaries closes sessions inactive for more than 180 days
// and clears their summary text.
func (retentionJob *Job) pruneExpiredSessionSummaries(ctx context.Context) {
	cutoff := time.Now().Add(-RetentionPolicy.RollingSummary)
	result := retentionJob.dbConnection.WithContext(ctx).
		Exec(`UPDATE chat_session SET status = 'closed', summary_text = NULL
              WHERE last_active_at < ? AND status = 'active'`, cutoff)
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("retention: failed to prune expired session summaries")
		return
	}
	logrus.WithField("rows_updated", result.RowsAffected).Info("retention: pruned expired session summaries")
}

// markStaleInferredMemoryAsPending moves inferred memory items that have not been
// confirmed in 12 months back to pending_confirmation status per PRD §8.3.
func (retentionJob *Job) markStaleInferredMemoryAsPending(ctx context.Context) {
	cutoff := time.Now().Add(-365 * 24 * time.Hour)
	result := retentionJob.dbConnection.WithContext(ctx).
		Exec(`UPDATE user_memory_item
              SET status = 'pending_confirmation'
              WHERE source = 'inferred'
                AND status = 'active'
                AND (last_confirmed_at IS NULL OR last_confirmed_at < ?)`, cutoff)
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("retention: failed to mark stale inferred memory")
		return
	}
	logrus.WithField("rows_updated", result.RowsAffected).Info("retention: marked stale inferred memory as pending_confirmation")
}
