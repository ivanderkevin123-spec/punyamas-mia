package content

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

var (
	ErrNotReviewable = errors.New("draf diblokir gerbang kode dan tidak dapat ditinjau; revisi dulu dengan mia content revise")
	ErrNoChange      = errors.New("tidak ada perubahan material pada teks draf")
)

// StatusLabels are the Indonesian labels of draft statuses.
var StatusLabels = map[string]string{
	domain.DraftDraft:       "draf — lolos gerbang kode, menunggu tinjauan manusia",
	domain.DraftNeedsReview: "perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa",
	domain.DraftApproved:    "disetujui peninjau untuk tahap berikutnya (bukan izin publikasi)",
	domain.DraftRejected:    "ditolak peninjau",
	domain.DraftBlocked:     "DIBLOKIR gerbang kode — tidak dapat ditinjau sebelum direvisi",
}

// Readiness tells whether a draft may be handed over to the manual publication-approval process.
// MIA itself never publishes; Ready only means nothing in MIA still blocks the handoff.
type Readiness struct {
	Ready   bool
	Reasons []string
}

// ReadinessOf evaluates handoff readiness from the stored draft and its review history.
func ReadinessOf(d *domain.ContentDraft, reviews []*domain.ApprovalRecord) Readiness {
	var rs []string
	dt := DecodeDetails(d)
	if d.Status != domain.DraftApproved {
		rs = append(rs, "status "+d.Status+": belum disetujui peninjau untuk versi ini")
	} else {
		var last *domain.ApprovalRecord
		if len(reviews) > 0 {
			last = reviews[len(reviews)-1]
		}
		if last == nil || last.Decision != domain.DraftApproved || last.SubjectVersion != d.Version || last.SubjectHash != d.ContentHash {
			rs = append(rs, "versi saat ini tidak cocok dengan keputusan persetujuan terakhir")
		}
	}
	if Hash(d.ContentType, d.Channel, d.Hook, d.Headline, d.Body, d.CallToAction) != d.ContentHash {
		rs = append(rs, "teks tersimpan tidak cocok dengan hash versinya (perubahan di luar MIA); revisi untuk mencatat versi baru")
	}
	if len(dt.Placeholders) > 0 {
		rs = append(rs, fmt.Sprintf("%d penanda isian belum diisi: %s", len(dt.Placeholders), strings.Join(dt.Placeholders, ", ")))
	}
	if c := dt.ClaimsToVerify(); len(c) > 0 {
		rs = append(rs, fmt.Sprintf("%d klaim perlu verifikasi", len(c)))
	}
	if n := dt.OpenFindings(); n > 0 {
		rs = append(rs, fmt.Sprintf("%d temuan gerbang belum diselesaikan", n))
	}
	return Readiness{Ready: len(rs) == 0, Reasons: rs}
}

// Review appends a human decision (approved|rejected) for the current version of a draft.
func Review(ctx context.Context, st *store.Store, gen ids.Generator, now time.Time, draftID, decision, note, by string) (*domain.ContentDraft, error) {
	if decision != domain.DraftApproved && decision != domain.DraftRejected {
		return nil, fmt.Errorf("keputusan %q tidak dikenal (approved|rejected)", decision)
	}
	if strings.TrimSpace(note) == "" {
		return nil, errors.New("catatan tinjauan wajib diisi")
	}
	if strings.TrimSpace(by) == "" {
		by = "CEO"
	}
	d, err := st.GetContentDraft(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.Status == domain.DraftBlocked {
		return nil, ErrNotReviewable
	}
	if Hash(d.ContentType, d.Channel, d.Hook, d.Headline, d.Body, d.CallToAction) != d.ContentHash {
		return nil, errors.New("teks draf berubah di luar MIA; revisi dengan mia content revise agar versinya tercatat sebelum ditinjau")
	}
	if decision == domain.DraftApproved {
		opp, err := st.GetOpportunity(ctx, d.OpportunityID)
		if err != nil {
			return nil, err
		}
		if opp.Status != domain.OppApproved {
			return nil, fmt.Errorf("peluang asal %s berstatus %s; draf tidak dapat disetujui", opp.ID, opp.Status)
		}
	}
	reviews, err := st.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
	if err != nil {
		return nil, err
	}
	if n := len(reviews); n > 0 && reviews[n-1].SubjectVersion == d.Version && reviews[n-1].Decision == decision {
		return nil, fmt.Errorf("keputusan %s sudah tercatat untuk versi %d", decision, d.Version)
	}
	ts := now.UTC().Format(time.RFC3339)
	rec := &domain.ApprovalRecord{ID: gen.New("appr"), SubjectType: domain.SubjectContentDraft, SubjectID: d.ID, Decision: decision, DecidedBy: by, Note: strings.TrimSpace(note),
		DecidedAt: ts, SubjectVersion: d.Version, SubjectHash: d.ContentHash}
	if err := st.ReviewContentDraft(ctx, rec, decision, ts); err != nil {
		return nil, err
	}
	return st.GetContentDraft(ctx, d.ID)
}

// Edit holds the fields a human revision changes; nil leaves a field as it is.
type Edit struct {
	Hook, Headline, Body, CallToAction *string
}

// Revise records a human revision: the text is re-checked by every gate, a new version is appended,
// and the draft returns to needs_review (or blocked when a hard gate fails). Evidence links are kept.
func Revise(ctx context.Context, st *store.Store, ev *Evaluator, cfg *config.Config, now time.Time, draftID string, edit Edit, note string) (*domain.ContentDraft, *Result, error) {
	if strings.TrimSpace(note) == "" {
		return nil, nil, errors.New("catatan revisi wajib diisi")
	}
	d, err := st.GetContentDraft(ctx, draftID)
	if err != nil {
		return nil, nil, err
	}
	src, err := Load(ctx, st, d.OpportunityID)
	if err != nil {
		return nil, nil, err
	}
	dt := DecodeDetails(d)
	ad := agent.Draft{ContentType: d.ContentType, Variant: d.Variant, Channel: d.Channel, TargetSegment: d.TargetSegment, Objective: d.Objective,
		Hook: d.Hook, Headline: d.Headline, Body: d.Body, CallToAction: d.CallToAction, BrandFactIDs: dt.BrandFactIDs, Assumptions: dt.Assumptions, Risks: dt.Risks}
	for _, f := range []struct {
		dst *string
		val *string
	}{{&ad.Hook, edit.Hook}, {&ad.Headline, edit.Headline}, {&ad.Body, edit.Body}, {&ad.CallToAction, edit.CallToAction}} {
		if f.val != nil {
			*f.dst = strings.TrimSpace(*f.val)
		}
	}
	links, err := st.ContentEvidenceFor(ctx, d.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, l := range links {
		if e, ok := src.ByID[l.ItemID]; ok {
			ad.SupportingEvidenceRefs = append(ad.SupportingEvidenceRefs, e.Ref)
		}
	}
	for _, c := range dt.Claims {
		ad.Claims = append(ad.Claims, agent.Claim{Text: c.Text, Kind: c.Kind, EvidenceRefs: c.EvidenceRefs, FactIDs: c.FactIDs})
	}
	for _, c := range dt.Checklist {
		if c.Source == "agent" {
			ad.ReviewChecklist = append(ad.ReviewChecklist, c.Text)
		}
	}
	if reasons := agent.CheckDraft(ad, cfg.Content, cfg.Scoring.Segments); len(reasons) > 0 {
		return nil, nil, fmt.Errorf("revisi tidak memenuhi kontrak %s: %s", agent.SchemaVersion, strings.Join(reasons, "; "))
	}
	if Hash(ad.ContentType, ad.Channel, ad.Hook, ad.Headline, ad.Body, ad.CallToAction) == d.ContentHash {
		return nil, nil, ErrNoChange
	}
	r := ev.Evaluate(ad, src)
	status := domain.DraftNeedsReview
	if r.Status == domain.DraftBlocked {
		status = domain.DraftBlocked
	}
	prev := d.Version
	ts := now.UTC().Format(time.RFC3339)
	d.Hook, d.Headline, d.Body, d.CallToAction = ad.Hook, ad.Headline, ad.Body, ad.CallToAction
	d.ClaimsJSON, d.PlaceholdersJSON, d.ChecklistJSON, d.FindingsJSON = mustJSON(r.Claims), mustJSON(r.Placeholders), mustJSON(r.Checklist), mustJSON(r.Findings)
	d.Status, d.Version, d.ContentHash, d.UpdatedAt = status, prev+1, r.ContentHash, ts
	v := &domain.ContentDraftVersion{DraftID: d.ID, Version: d.Version, Hook: d.Hook, Headline: d.Headline, Body: d.Body, CallToAction: d.CallToAction, ContentHash: d.ContentHash,
		Author: "human", Note: strings.TrimSpace(note), StatusAfter: status, FindingsJSON: d.FindingsJSON, PlaceholdersJSON: d.PlaceholdersJSON, CreatedAt: ts}
	if err := st.ReviseContentDraft(ctx, d, v, prev); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, nil, errors.New("teks revisi identik dengan draf lain untuk peluang, jenis, dan kanal yang sama")
		}
		return nil, nil, err
	}
	r.Status = status
	updated, err := st.GetContentDraft(ctx, d.ID)
	return updated, &r, err
}
