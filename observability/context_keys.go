package observability

// RequestIDKey is the context key for HTTP webhook request IDs.
type RequestIDKey struct{}

func (RequestIDKey) Key() string { return "request_id" }

// RunIDKey is the context key for cron/CLI run IDs.
type RunIDKey struct{}

func (RunIDKey) Key() string { return "run_id" }

// CommandKey is the context key for the subcommand name.
type CommandKey struct{}

func (CommandKey) Key() string { return "command" }
