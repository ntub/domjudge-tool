package importer

import "log/slog"

func logDecision(logger *slog.Logger, op Op, path Path, reason string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("router decision", "op", op.String(), "path", string(path), "reason", reason)
}
