// Package domain holds the entity types and enumerations shared by the store,
// orchestrator and reports. Timestamps are RFC 3339 strings in UTC unless noted.
package domain

// Run statuses.
const (
	RunCreated               = "created"
	RunRunning               = "running"
	RunCompleted             = "completed"
	RunCompletedWithWarnings = "completed_with_warnings"
	RunPartial               = "partial"
	RunFailed                = "failed"
	RunStale                 = "stale"
)

// Step statuses.
const (
	StepPending         = "pending"
	StepRunning         = "running"
	StepCompleted       = "completed"
	StepFailed          = "failed"
	StepTimedOut        = "timed_out"
	StepBudgetExhausted = "budget_exhausted"
	StepSkipped         = "skipped"
)

// Item kinds (truth levels).
const (
	KindFact           = "fact"
	KindInterpretation = "interpretation"
	KindHypothesis     = "hypothesis"
	KindRecommendation = "recommendation"
)

// Item origins.
const (
	OriginImport    = "import"
	OriginWorkspace = "workspace"
	OriginFetch     = "fetch"
	OriginAgent     = "agent"
)

// Item statuses.
const (
	ItemImported  = "imported"
	ItemValidated = "validated"
	ItemRejected  = "rejected"
	ItemDuplicate = "duplicate"
	ItemConflict  = "conflict"
)

// Source types.
const (
	SourceManualNote      = "manual_note"
	SourceImportedFile    = "imported_file"
	SourceInternalWS      = "internal_workspace"
	SourceWebPage         = "web_page"
	SourceTrendExport     = "trend_export"
	SourceAnalyticsExport = "analytics_export"
)

// Asset review statuses.
const (
	AssetAccepted       = "accepted"
	AssetReviewRequired = "review_required"
	AssetMissing        = "missing"
)

type Run struct {
	ID             string
	Kind           string
	Status         string
	CreatedAt      string
	StartedAt      string
	FinishedAt     string
	HeartbeatAt    string
	ConfigHash     string
	ProviderName   string
	InputsJSON     string
	SummaryJSON    string
	CostUSDTotal   *float64
	CostMeasurable bool
	ParentRunID    string
}

type Step struct {
	ID         string
	RunID      string
	Name       string
	Ordinal    int
	Status     string
	Attempts   int
	StartedAt  string
	FinishedAt string
	ErrorJSON  string
}

type Source struct {
	ID             string
	SourceType     string
	Name           string
	Reference      string
	CanonicalURL   string
	Publisher      string
	RetrievedAt    string
	PublishedAt    string
	ContentHash    string
	SnapshotPath   string
	Status         string
	StatusReason   string
	AllowlistEntry string
	CreatedAt      string
}

type Item struct {
	ID               string
	RunID            string
	Ref              string
	Origin           string
	Kind             string
	TopicKey         string
	Title            string
	Statement        string
	Rationale        string
	Language         string
	Quality          string
	Status           string
	ContentHash      string
	DuplicateOf      string
	ConflictGroup    string
	ClaimCheckJSON   string
	Suspicious       bool
	AgentExecutionID string
	CreatedAt        string
}

type Evidence struct {
	ID          string
	ItemID      string
	SourceID    string
	FromItemID  string
	Excerpt     string
	Locator     string
	RetrievedAt string
	PublishedAt string
	Note        string
}

type ImportedAsset struct {
	ID                string
	AssetKey          string
	Root              string
	RelativePath      string
	SHA256            string
	SizeBytes         int64
	MTime             string
	WorkspaceCommit   string
	ImportedAt        string
	ParsedSummaryJSON string
	ReviewStatus      string
	AcceptedAt        string
	AcceptedByNote    string
}

type AgentExecution struct {
	ID              string
	RunID           string
	StepID          string
	AgentName       string
	ProviderName    string
	Model           string
	SchemaVersion   string
	RequestHash     string
	RequestPath     string
	ResponsePath    string
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	CostUSD         *float64
	CostMeasurable  bool
	LatencyMS       int64
	Attempt         int
	Outcome         string
	ErrorText       string
	CreatedAt       string
}

type CostRecord struct {
	ID               string
	RunID            string
	AgentExecutionID string
	AgentName        string
	Category         string
	ProviderName     string
	UnitsJSON        string
	UnitPriceSource  string
	CostUSD          *float64
	Estimated        bool
	PeriodMonth      string
	CreatedAt        string
}

type ApprovalRecord struct {
	ID             string
	SubjectType    string
	SubjectID      string
	Decision       string
	DecidedBy      string
	Note           string
	DecidedAt      string
	SubjectVersion int    // versioned subjects (content drafts): version the decision applies to; 0 = not versioned
	SubjectHash    string // content hash of that version
}

// RunSummary is stored as summary_json on the run and rendered in reports.
type RunSummary struct {
	RunID          string        `json:"run_id"`
	Kind           string        `json:"kind"`
	Status         string        `json:"status"`
	Steps          []StepSummary `json:"steps"`
	Counts         Counts        `json:"counts"`
	Warnings       []string      `json:"warnings"`
	Errors         []string      `json:"errors"`
	Drift          []DriftEntry  `json:"drift"`
	CostUSD          float64       `json:"cost_usd"`
	CostActualUSD    float64       `json:"cost_actual_usd"`
	CostUncertainUSD float64       `json:"cost_uncertain_usd"`
	CostMeasurable   bool          `json:"cost_measurable"`
	ReportMarkdown    string          `json:"report_markdown,omitempty"`
	ReportHTML        string          `json:"report_html,omitempty"`
	DurationS         float64         `json:"duration_s"`
	ParentRunID       string          `json:"parent_run_id,omitempty"`
	Strategy          *StrategyCounts `json:"strategy,omitempty"`
	Content           *ContentCounts  `json:"content,omitempty"`
	ContentConfigHash string          `json:"content_config_hash,omitempty"`
	OpportunityID     string          `json:"opportunity_id,omitempty"`
	FixtureHash       string          `json:"fixture_hash,omitempty"`
	ScoringConfigHash string          `json:"scoring_config_hash,omitempty"`
}

type StepSummary struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
	Error    string `json:"error,omitempty"`
}

type Counts struct {
	FilesImported   int `json:"files_imported"`
	FilesRejected   int `json:"files_rejected"`
	Sources         int `json:"sources"`
	ItemsImported   int `json:"items_imported"`
	ItemsReused     int `json:"items_reused"`
	Facts           int `json:"facts"`
	Interpretations int `json:"interpretations"`
	Hypotheses      int `json:"hypotheses"`
	Duplicates      int `json:"duplicates"`
	Suspicious      int `json:"suspicious"`
	Rejected        int `json:"rejected"`
	AgentFindings   int `json:"agent_findings"`
	AgentRejected   int `json:"agent_rejected"`
	ClaimFlags      int `json:"claim_flags"`
}

type DriftEntry struct {
	Key     string   `json:"key"`
	Status  string   `json:"status"` // unchanged | changed | missing | new | review_required
	Path    string   `json:"path"`
	Details []string `json:"details,omitempty"`
}

// Opportunity statuses (Phase 3). Review decisions never overwrite earlier
// approval_records rows; the status column mirrors the latest decision.
const (
	OppProposed         = "proposed"           // passed the gates; awaiting human review
	OppBlocked          = "blocked"            // failed brand validation; cannot be approved
	OppRejected         = "rejected"           // failed evidence or factual-claim gates
	OppApproved         = "approved"           // human decision
	OppNeedsEvidence    = "needs_more_evidence" // human decision
	OppRejectedByReview = "rejected_by_review" // human decision
)

// Tiers.
const (
	TierA    = "A"
	TierB    = "B"
	TierC    = "C"
	TierNone = "none"
)

type Opportunity struct {
	ID                string
	RunID             string
	ParentRunID       string
	ProposalIndex     int
	Title             string
	Segment           string
	TopicKey          string
	Need              string
	Interpretation    string
	Status            string
	Tier              string
	Hypothesis        bool
	RequiresStore     bool
	DimensionsJSON    string
	OrdinalSum        int
	CapsJSON          string
	GatesJSON         string
	ConflictsJSON     string
	BriefJSON         string
	MergedFromJSON    string
	RejectReasonsJSON string
	ConfigHash        string
	ScoringConfigHash string
	FixtureHash       string
	AgentExecutionID  string
	CreatedAt         string
}

type OpportunityEvidence struct {
	OpportunityID string
	ItemID        string
	Role          string // fact | interpretation | hypothesis
}

type StrategyCounts struct {
	Proposed       int `json:"proposed"`        // proposals received from the agent
	Merged         int `json:"merged"`          // proposals merged into another (Gate 5)
	Evaluated      int `json:"evaluated"`       // opportunities stored
	Blocked        int `json:"blocked"`         // brand validation failures
	Rejected       int `json:"rejected"`        // evidence or factual-claim failures
	TierA          int `json:"tier_a"`
	TierB          int `json:"tier_b"`
	TierC          int `json:"tier_c"`
	Hypotheses     int `json:"hypotheses"`
	Conflicted     int `json:"conflicted"`
	StoreDependent int `json:"store_dependent"`
	EvidenceItems  int `json:"evidence_items"` // validated items available from the research run
	ConflictGroups int `json:"conflict_groups"`
}

// Content draft statuses (Phase 4). A draft is never published by MIA; "approved" means a
// human approved this version for the next manual stage, not for publication.
const (
	DraftDraft       = "draft"        // passed every code gate; awaits human review
	DraftNeedsReview = "needs_review" // placeholders, claims to verify, review-level findings, or changed after review
	DraftApproved    = "approved"     // human decision for the current version
	DraftRejected    = "rejected"     // human decision
	DraftBlocked     = "blocked"      // a hard gate failed; cannot be reviewed until revised
)

// SubjectContentDraft is the approval_records subject type of content draft reviews.
const SubjectContentDraft = "content_draft"

type ContentDraft struct {
	ID                string
	RunID             string
	OpportunityID     string
	StrategyRunID     string
	ResearchRunID     string
	ProposalIndex     int
	ContentType       string
	Variant           int
	Channel           string
	TargetSegment     string
	Objective         string
	Hook              string
	Headline          string
	Body              string
	CallToAction      string
	ClaimsJSON        string
	PlaceholdersJSON  string
	AssumptionsJSON   string
	RisksJSON         string
	ChecklistJSON     string
	FindingsJSON      string
	BrandFactIDsJSON  string
	Status            string
	Version           int
	ContentHash       string
	ExecutionID       string
	ConfigHash        string
	ContentConfigHash string
	ScoringConfigHash string
	FixtureHash       string
	CreatedAt         string
	UpdatedAt         string
}

type ContentDraftEvidence struct {
	DraftID       string
	OpportunityID string
	ItemID        string
	Role          string // fact | interpretation | hypothesis
}

type ContentDraftVersion struct {
	DraftID          string
	Version          int
	Hook             string
	Headline         string
	Body             string
	CallToAction     string
	ContentHash      string
	Author           string // agent | human
	Note             string
	StatusAfter      string
	FindingsJSON     string
	PlaceholdersJSON string
	CreatedAt        string
}

// ContentCounts summarises a content run; storage-derived except the agent-output counters.
type ContentCounts struct {
	Proposed           int `json:"proposed"`            // drafts in the agent output
	StructuralRejected int `json:"structural_rejected"` // rejected by the content_drafts.v1 structural rules
	DuplicatesSkipped  int `json:"duplicates_skipped"`  // near-duplicates not stored
	Stored             int `json:"stored"`
	Draft              int `json:"draft"`
	NeedsReview        int `json:"needs_review"`
	Approved           int `json:"approved"`
	Rejected           int `json:"rejected"`
	Blocked            int `json:"blocked"`
	Placeholders       int `json:"placeholders"`     // [ISI: ...] markers across stored drafts
	ClaimsToVerify     int `json:"claims_to_verify"` // claims without supporting evidence across stored drafts
	EvidenceLinks      int `json:"evidence_links"`
}

// Workflow statuses (Phase 5). A workflow status summarises orchestration; the statuses of runs,
// opportunities and drafts stay authoritative.
const (
	WfCreated                   = "created"                     // research not finished yet; automatic step available
	WfResearchCompleted         = "research_completed"          // research passed gate A; strategy next
	WfStrategyCompleted         = "strategy_completed"          // approved opportunities without content; content generation next
	WfAwaitingOpportunityReview = "awaiting_opportunity_review" // opportunities wait for a human decision
	WfAwaitingContentReview     = "awaiting_content_review"     // drafts wait for review, revision or placeholders
	WfCompleted                 = "completed"                   // at least one draft ready for handoff and nothing pending
	WfBlocked                   = "blocked"                     // a quality gate or the budget stops the workflow
	WfFailed                    = "failed"                      // a stage run failed or was interrupted; resume is possible
)

type Workflow struct {
	ID                 string
	Status             string
	InputsDir          string
	FixtureKey         string
	ResearchRunID      string
	StrategyRunID      string
	LastCompletedStage string
	FailedStage        string
	Reason             string
	SummaryJSON        string
	ConfigHash         string
	LeaseOwner         string
	LeaseExpiresAt     string
	CreatedAt          string
	UpdatedAt          string
}

type WorkflowContentRun struct {
	WorkflowID    string
	OpportunityID string
	Attempt       int
	ContentRunID  string
	Origin        string // workflow | adopted
	CreatedAt     string
}

type WorkflowEvent struct {
	WorkflowID string
	Seq        int
	Kind       string
	Stage      string
	RefID      string
	Detail     string
	CreatedAt  string
}
