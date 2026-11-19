package channel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDuplicateOpportunity = errors.New("opportunity_exists")

type opportunityGap struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
}

type reverseMatch struct {
	PartnerID     string `json:"partnerId"`
	OpportunityID string `json:"opportunityId"`
}

type opportunityTask struct {
	OpportunityID string `json:"opportunityId"`
	ProjectTitle  string `json:"projectTitle"`
	Reason        string `json:"reason"`
	Action        string `json:"action"`
	Date          string `json:"date"`
}

type OpportunityDesk struct {
	Items          []OpportunityView `json:"items"`
	Partners       []channelPartner  `json:"partners"`
	Gaps           []opportunityGap  `json:"gaps"`
	ReverseMatches []reverseMatch    `json:"reverseMatches"`
	Tasks          []opportunityTask `json:"tasks"`
}

func dealer(actor data.User) bool {
	return actor.Organization.Kind == data.Reseller
}

func dealerProject(tx *gorm.DB, actor data.User, id uint64) (channelProject, error) {
	var project channelProject
	err := visibleProjects(tx, actor).First(&project, id).Error
	return project, err
}

func opportunities(tx *gorm.DB, actor data.User) *gorm.DB {
	return tx.Model(&channelOpportunity{}).
		Select("pl_channel_opportunities.*, p.title AS project_title").
		Joins("JOIN pl_channel_projects p ON p.id = pl_channel_opportunities.project_id AND p.reseller_id = pl_channel_opportunities.organization_id").
		Where("pl_channel_opportunities.organization_id = ?", actor.OrganizationID)
}

func loadPartners(tx *gorm.DB, organizationID uint64) ([]channelPartner, error) {
	partners := []channelPartner{}
	if err := tx.Where("organization_id = ?", organizationID).Order("id DESC").Find(&partners).Error; err != nil {
		return nil, err
	}
	if len(partners) == 0 {
		return partners, nil
	}
	var records []channelPartnerCase
	ids := tx.Model(&channelPartner{}).Select("id").Where("organization_id = ?", organizationID)
	if err := tx.Where("partner_id IN (?)", ids).Order("opportunity_id").Find(&records).Error; err != nil {
		return nil, err
	}
	evidence := map[uint64][]string{}
	for _, record := range records {
		line := "团队引荐 " + idString(record.OpportunityID) + " 已促成" + categories[record.Category] + "合作（成交记录，非交付验收）。"
		evidence[record.PartnerID] = append(evidence[record.PartnerID], line)
	}
	for i := range partners {
		p := &partners[i]
		if addition := strings.Join(evidence[p.ID], "\n"); addition != "" {
			if p.Evidence != "" {
				p.Evidence += "\n"
			}
			p.Evidence += addition
		}
	}
	return partners, nil
}

func (s Service) Desk(ctx context.Context, actor data.User) (OpportunityDesk, error) {
	if !dealer(actor) {
		return OpportunityDesk{}, ErrForbidden
	}
	desk := OpportunityDesk{
		Items: []OpportunityView{}, Gaps: []opportunityGap{}, ReverseMatches: []reverseMatch{}, Tasks: []opportunityTask{},
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		desk.Partners, err = loadPartners(tx, actor.OrganizationID)
		if err != nil {
			return err
		}
		var items []channelOpportunity
		if err := opportunities(tx, actor).Order("pl_channel_opportunities.id DESC").Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		histories, known, err := opportunityContext(tx, actor.ID, ids)
		if err != nil {
			return err
		}
		counts := map[string]int{}
		today := time.Now().In(chinaTime).Format(time.DateOnly)
		for _, item := range items {
			view := opportunityView(item, desk.Partners, histories[item.ID], known[item.ID])
			desk.Items = append(desk.Items, view)
			if item.Stage == opportunityOpen {
				if item.Category != "" {
					counts[item.Category]++
				}
				for _, match := range view.Matches {
					desk.ReverseMatches = append(desk.ReverseMatches, reverseMatch{match.PartnerID, view.ID})
				}
			}
			if task, ok := taskFor(view, today); ok {
				desk.Tasks = append(desk.Tasks, task)
			}
		}
		for category, count := range counts {
			desk.Gaps = append(desk.Gaps, opportunityGap{category, categories[category], count})
		}
		slices.SortFunc(desk.Gaps, func(a, b opportunityGap) int {
			if a.Count != b.Count {
				return b.Count - a.Count
			}
			return strings.Compare(a.Category, b.Category)
		})
		slices.SortStableFunc(desk.Tasks, func(a, b opportunityTask) int {
			if (a.Date == "") != (b.Date == "") {
				if a.Date == "" {
					return 1
				}
				return -1
			}
			return strings.Compare(a.Date, b.Date)
		})
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return desk, err
}

func opportunityContext(tx *gorm.DB, userID uint64, ids []uint64) (map[uint64][]opportunityHistory, map[uint64][]string, error) {
	var events []channelOpportunityEvent
	if err := tx.Select("opportunity_id", "at", "label").Where("opportunity_id IN ?", ids).Order("id").Find(&events).Error; err != nil {
		return nil, nil, err
	}
	histories := map[uint64][]opportunityHistory{}
	for _, event := range events {
		histories[event.OpportunityID] = append(histories[event.OpportunityID], opportunityHistory{event.At, event.Label})
	}
	var rows []channelOpportunityRead
	if err := tx.Where("opportunity_id IN ? AND user_id = ?", ids, userID).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	known := map[uint64][]string{}
	for _, row := range rows {
		known[row.OpportunityID] = row.PartnerIDs
	}
	return histories, known, nil
}

func taskFor(item OpportunityView, today string) (opportunityTask, bool) {
	task := opportunityTask{OpportunityID: item.ID, ProjectTitle: item.ProjectTitle, Date: item.FollowUp}
	switch {
	case item.Stage == opportunityWon && positiveFee(item.FeeAmount):
		task.Reason, task.Action = "合作已促成，需核对介绍费条件（"+item.FeeTerms+"）", "核对结清条件"
	case item.Stage != opportunityOpen && item.Stage != opportunityIntroduced:
		return task, false
	case item.FollowUp != "" && item.FollowUp <= today:
		label := map[string]string{"customer": "客户约定答复", "partner": "伙伴回访", "fee": "介绍费核对"}[item.FollowReason]
		task.Reason, task.Action = label+"已到日期 "+item.FollowUp, "记录跟进结果"
	case item.Stage == opportunityOpen:
		names := []string{}
		for _, match := range item.Matches {
			if !slices.Contains(item.KnownPartnerIDs, match.PartnerID) {
				names = append(names, match.Name)
			}
		}
		if len(names) != 0 {
			task.Reason, task.Action = "新匹配到 "+strings.Join(names, "、")+"，可以回访此需求", "查看新伙伴依据"
		} else if len(item.Questions) != 0 {
			task.Reason, task.Action = "需要确认承接类型或涉及系统", "补充需求信息"
		}
	}
	return task, task.Action != ""
}

func (s Service) CaptureOpportunity(ctx context.Context, actor data.User, input CaptureInput) (OpportunityView, error) {
	if !dealer(actor) {
		return OpportunityView{}, ErrForbidden
	}
	projectID, err := parseID(input.ProjectID)
	input.Payload.Note = strings.TrimSpace(input.Payload.Note)
	if err != nil || !requestIDPattern.MatchString(input.RequestID) || !fits(input.Payload.Note, 2000) || !input.Payload.Capability.valid(false) {
		return OpportunityView{}, ErrInvalid
	}
	var result OpportunityView
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, err := dealerProject(tx, actor, projectID)
		if err != nil {
			return err
		}
		entry, replay, err := claim(tx, actor.OrganizationID, "opportunity:capture:"+idString(projectID)+":"+idString(actor.ID), input.RequestID, input.Payload)
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &result)
		}
		item, err := createOpportunity(tx, actor, project, input.Payload, false)
		if err != nil {
			return err
		}
		result, err = loadOpportunity(tx, actor, item)
		if err != nil {
			return err
		}
		return saveResult(tx, entry, &projectID, result)
	})
	return result, err
}

func createOpportunity(tx *gorm.DB, actor data.User, project channelProject, payload OpportunityInput, reuse bool) (channelOpportunity, error) {
	sum := sha256.Sum256([]byte(payload.Note))
	item := channelOpportunity{
		OrganizationID: actor.OrganizationID, ProjectID: project.ID, ProjectTitle: project.Title, NoteHash: hex.EncodeToString(sum[:]),
		OpportunityInput: payload, Revision: 1, Stage: opportunityOpen, ContactReviews: map[string]contactReview{}, Rejections: []rejection{},
	}
	if err := tx.Create(&item).Error; err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return item, err
		}
		if !reuse {
			return item, ErrDuplicateOpportunity
		}
		var previous channelOpportunity
		err := opportunities(tx, actor).Clauses(clause.Locking{Strength: "SHARE"}).
			Where("pl_channel_opportunities.project_id = ? AND note_hash = ?", project.ID, item.NoteHash).Take(&previous).Error
		return previous, err
	}
	if err := addOpportunityEvent(tx, actor.ID, item.ID, "capture", "确认记录附加需求", struct{}{}); err != nil {
		return item, err
	}
	partners, err := loadPartners(tx, actor.OrganizationID)
	if err != nil {
		return item, err
	}
	return item, markOpportunityRead(tx, item.ID, actor.ID, matchesFor(item, partners))
}

func (s Service) AddPartner(ctx context.Context, actor data.User, input AddPartnerInput) (channelPartner, error) {
	if !dealer(actor) {
		return channelPartner{}, ErrForbidden
	}
	p := input.Payload
	p.Name, p.Owner, p.Evidence = strings.TrimSpace(p.Name), strings.TrimSpace(p.Owner), strings.TrimSpace(p.Evidence)
	if !requestIDPattern.MatchString(input.RequestID) || !fits(p.Name, 80) || !fits(p.Owner, 60) ||
		p.Evidence != "" && !fits(p.Evidence, 500) || !p.Capability.valid(true) {
		return channelPartner{}, ErrInvalid
	}
	var result channelPartner
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entry, replay, err := claim(tx, actor.OrganizationID, "opportunity:partner", input.RequestID, p)
		if err != nil {
			return err
		}
		if replay {
			return json.Unmarshal(entry.Result, &result)
		}
		result = channelPartner{OrganizationID: actor.OrganizationID, CreatedBy: actor.ID, PartnerInput: p}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return saveResult(tx, entry, nil, result)
	})
	return result, err
}

func loadOpportunity(tx *gorm.DB, actor data.User, item channelOpportunity) (OpportunityView, error) {
	partners, err := loadPartners(tx, actor.OrganizationID)
	if err != nil {
		return OpportunityView{}, err
	}
	history, known, err := opportunityContext(tx, actor.ID, []uint64{item.ID})
	if err != nil {
		return OpportunityView{}, err
	}
	return opportunityView(item, partners, history[item.ID], known[item.ID]), nil
}

func addOpportunityEvent(tx *gorm.DB, actorID, opportunityID uint64, action, label string, detail any) error {
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return tx.Create(&channelOpportunityEvent{
		OpportunityID: opportunityID, ActorUserID: actorID, At: time.Now().UTC(), Action: action, Label: label, Detail: body,
	}).Error
}

func markOpportunityRead(tx *gorm.DB, opportunityID, userID uint64, matches []matchView) error {
	known := make([]string, 0, len(matches))
	for _, match := range matches {
		known = append(known, match.PartnerID)
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "opportunity_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"partner_ids"}),
	}).Create(&channelOpportunityRead{OpportunityID: opportunityID, UserID: userID, PartnerIDs: known}).Error
}
