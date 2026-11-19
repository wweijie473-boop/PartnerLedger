package channel

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type channelVisit struct {
	ID             uint64
	OrganizationID uint64
	OwnerID        uint64
	ProjectID      *uint64
	OpportunityID  *uint64
	Text           string
	Main           string
	Additional     string
	Confirmed      bool
	Reviewed       bool
	Revision       uint32
	UpdatedAt      time.Time
	RemovedAt      *time.Time
}

type channelVisitEvent struct {
	ID          uint64
	VisitID     uint64
	ActorUserID uint64
	At          time.Time
	Action      string
	Revision    uint32
}

type VisitRevisionInput struct {
	RequestID string `json:"requestId"`
	Revision  uint32 `json:"revision"`
}

type VisitDraft struct {
	ProjectID  string `json:"projectId"`
	Text       string `json:"text"`
	Main       string `json:"main"`
	Additional string `json:"additional"`
	Reviewed   bool   `json:"reviewed"`
}

type SaveVisitInput struct {
	VisitRevisionInput
	Payload VisitDraft `json:"payload"`
}

type ConfirmVisitInput struct {
	VisitRevisionInput
	Capability
	Confirmed         bool `json:"confirmed"`
	CreateOpportunity bool `json:"createOpportunity"`
}

type VisitView struct {
	ID string `json:"id"`
	VisitDraft
	OpportunityID string    `json:"opportunityId"`
	Confirmed     bool      `json:"confirmed"`
	Revision      uint32    `json:"revision"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type DeletedVisit struct {
	ID       string `json:"id"`
	Revision uint32 `json:"revision"`
	Deleted  bool   `json:"deleted"`
}

func visitView(v channelVisit) VisitView {
	return VisitView{
		ID:            idString(v.ID),
		VisitDraft:    VisitDraft{optionalID(v.ProjectID), v.Text, v.Main, v.Additional, v.Reviewed},
		OpportunityID: optionalID(v.OpportunityID), Confirmed: v.Confirmed, Revision: v.Revision, UpdatedAt: v.UpdatedAt,
	}
}

func ownVisits(tx *gorm.DB, actor data.User) *gorm.DB {
	projects := visibleProjects(tx, actor).Select("id")
	return tx.Model(&channelVisit{}).Where("organization_id = ? AND owner_id = ?", actor.OrganizationID, actor.ID).
		Where("(project_id IS NULL OR project_id IN (?))", projects)
}

func (s Service) Visits(ctx context.Context, actor data.User) ([]VisitView, error) {
	if !dealer(actor) {
		return nil, ErrForbidden
	}
	var rows []channelVisit
	if err := ownVisits(s.db.WithContext(ctx), actor).Where("removed_at IS NULL").Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	views := make([]VisitView, 0, len(rows))
	for _, row := range rows {
		views = append(views, visitView(row))
	}
	return views, nil
}

func (s Service) Visit(ctx context.Context, actor data.User, id uint64) (VisitView, error) {
	if !dealer(actor) {
		return VisitView{}, ErrForbidden
	}
	var row channelVisit
	err := ownVisits(s.db.WithContext(ctx), actor).Where("removed_at IS NULL").First(&row, id).Error
	return visitView(row), err
}

func validateVisitDraft(draft VisitDraft) (VisitDraft, *uint64, error) {
	draft.Text, draft.Main, draft.Additional = strings.TrimSpace(draft.Text), strings.TrimSpace(draft.Main), strings.TrimSpace(draft.Additional)
	if !fits(draft.Text, 2000) || utf8.RuneCountInString(draft.Main) > 2000 || utf8.RuneCountInString(draft.Additional) > 2000 {
		return draft, nil, ErrInvalid
	}
	if draft.ProjectID == "" {
		return draft, nil, nil
	}
	id, err := parseID(draft.ProjectID)
	return draft, &id, err
}

func (s Service) SaveVisit(ctx context.Context, actor data.User, id uint64, input SaveVisitInput) (VisitView, error) {
	if !dealer(actor) {
		return VisitView{}, ErrForbidden
	}
	draft, projectID, err := validateVisitDraft(input.Payload)
	if err != nil || !requestIDPattern.MatchString(input.RequestID) || (id == 0) != (input.Revision == 0) {
		return VisitView{}, ErrInvalid
	}
	var result VisitView
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if projectID != nil {
			if _, err := dealerProject(tx, actor, *projectID); err != nil {
				return err
			}
		}
		row := channelVisit{OrganizationID: actor.OrganizationID, OwnerID: actor.ID}
		if id != 0 {
			if err := ownVisits(tx, actor).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
				return err
			}
		}
		operation := "visit:save:" + idString(actor.ID) + ":" + idString(id)
		entry, replay, err := claim(tx, actor.OrganizationID, operation, input.RequestID, struct {
			Revision uint32
			Payload  VisitDraft
		}{input.Revision, draft})
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &result)
		}
		if row.RemovedAt != nil {
			return ErrNotFound
		}
		if input.Revision != row.Revision {
			return ErrRevision
		}
		if row.OpportunityID != nil {
			return ErrTransition
		}
		row.ProjectID, row.Text, row.Main, row.Additional = projectID, draft.Text, draft.Main, draft.Additional
		row.Reviewed, row.Confirmed = draft.Reviewed, false
		row.Revision++
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := auditVisit(tx, actor.ID, row, "save"); err != nil {
			return err
		}
		result = visitView(row)
		return saveResult(tx, entry, projectID, result)
	})
	return result, err
}

func (s Service) ConfirmVisit(ctx context.Context, actor data.User, id uint64, input ConfirmVisitInput) (VisitView, error) {
	if !dealer(actor) {
		return VisitView{}, ErrForbidden
	}
	if !input.Confirmed || !input.Capability.valid(false) {
		return VisitView{}, ErrInvalid
	}
	var result VisitView
	err := s.changeVisit(ctx, actor, id, "confirm", input.VisitRevisionInput, input, &result, func(tx *gorm.DB, row *channelVisit) error {
		if !row.Reviewed || row.ProjectID == nil || row.Main == "" && row.Additional == "" ||
			row.OpportunityID != nil || row.Confirmed && !input.CreateOpportunity {
			return ErrTransition
		}
		if input.CreateOpportunity {
			if row.Additional == "" {
				return ErrInvalid
			}
			project, err := dealerProject(tx, actor, *row.ProjectID)
			if err != nil {
				return err
			}
			opportunity, err := createOpportunity(tx, actor, project, OpportunityInput{input.Capability, row.Additional}, true)
			if err != nil {
				return err
			}
			row.OpportunityID = &opportunity.ID
		}
		row.Confirmed = true
		return nil
	}, func(row channelVisit) { result = visitView(row) })
	return result, err
}

func (s Service) DeleteVisit(ctx context.Context, actor data.User, id uint64, input VisitRevisionInput) (DeletedVisit, error) {
	var result DeletedVisit
	err := s.changeVisit(ctx, actor, id, "delete", input, input, &result, func(_ *gorm.DB, row *channelVisit) error {
		now := time.Now().UTC()
		row.RemovedAt = &now
		return nil
	}, func(row channelVisit) { result = DeletedVisit{idString(row.ID), row.Revision, true} })
	return result, err
}

func (s Service) changeVisit(ctx context.Context, actor data.User, id uint64, action string, key VisitRevisionInput,
	input, result any, apply func(*gorm.DB, *channelVisit) error, project func(channelVisit)) error {
	if !dealer(actor) {
		return ErrForbidden
	}
	if !requestIDPattern.MatchString(key.RequestID) || key.Revision == 0 {
		return ErrInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row channelVisit
		if err := ownVisits(tx, actor).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return err
		}
		operation := "visit:" + action + ":" + idString(actor.ID) + ":" + idString(id)
		entry, replay, err := claim(tx, actor.OrganizationID, operation, key.RequestID, input)
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, result)
		}
		if row.RemovedAt != nil {
			return ErrNotFound
		}
		if key.Revision != row.Revision {
			return ErrRevision
		}
		if err := apply(tx, &row); err != nil {
			return err
		}
		row.Revision++
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := auditVisit(tx, actor.ID, row, action); err != nil {
			return err
		}
		project(row)
		return saveResult(tx, entry, row.ProjectID, result)
	})
}

func auditVisit(tx *gorm.DB, actorID uint64, row channelVisit, action string) error {
	return tx.Create(&channelVisitEvent{VisitID: row.ID, ActorUserID: actorID, At: time.Now().UTC(), Action: action, Revision: row.Revision}).Error
}
