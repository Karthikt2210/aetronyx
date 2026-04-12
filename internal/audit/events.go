package audit

// M1 audit event type constants as defined in §6.2 of the master architecture doc.
const (
	EventChainGenesis       = "chain.genesis"
	EventRunCreated         = "run.created"
	EventRunStarted         = "run.started"
	EventRunCompleted       = "run.completed"
	EventRunFailed          = "run.failed"
	EventIterationStarted   = "iteration.started"
	EventIterationCompleted = "iteration.completed"
	EventIterationFailed    = "iteration.failed"
	EventLLMRequest         = "llm.request"
	EventLLMResponse        = "llm.response"
	EventFileRead           = "file.read"
	EventFileWrite          = "file.write"
	EventSpecValidated      = "spec.validated"
	EventSpecRejected       = "spec.rejected"
)

// Actor values for audit events.
const (
	ActorUser   = "user"
	ActorAgent  = "agent"
	ActorSystem = "system"
)

// Event mirrors the audit_events table exactly.
// All hash and signature fields are lowercase hex strings.
type Event struct {
	ID          string         // ULID
	RunID       *string        // nullable — genesis event has run_id set, others always have it
	IterationID *string        // nullable
	Ts          int64          // unix milliseconds UTC
	EventType   string         // see constants above
	Actor       string         // user|agent|system
	PayloadJSON []byte         // canonical JSON of the payload
	PayloadHash string         // hex-encoded sha256 of PayloadJSON
	PrevHash    string         // hex-encoded sha256 chain link (64 hex chars = 32 bytes)
	Signature   string         // hex-encoded ed25519 signature
	OtelTraceID *string        // optional OTel trace id
	OtelSpanID  *string        // optional OTel span id
}

// Payload is an untyped map used for building event bodies before marshalling.
type Payload map[string]any
