package channel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalid       = errors.New("invalid_request")
	ErrForbidden     = errors.New("forbidden")
	ErrRevision      = errors.New("revision_conflict")
	ErrTransition    = errors.New("invalid_transition")
	ErrIdempotency   = errors.New("idempotency_conflict")
	ErrLinkRequired  = errors.New("organization_link_required")
	ErrNotFound      = gorm.ErrRecordNotFound
	requestIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) Service {
	return Service{db: db}
}

type Page struct {
	Items      []View  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type CreateInput struct {
	RequestID   string `json:"requestId"`
	Title       string `json:"title"`
	Requirement string `json:"requirement"`
	DueDate     string `json:"dueDate"`
	CustomerID  string `json:"customerId"`
	ResellerID  string `json:"resellerId"`
}

type ActionInput struct {
	RequestID string          `json:"requestId"`
	Revision  uint32          `json:"revision"`
	Payload   json.RawMessage `json:"payload"`
}

func visibleProjects(db *gorm.DB, actor data.User) *gorm.DB {
	query := db.Model(&channelProject{})
	switch actor.Organization.Kind {
	case data.Customer:
		return query.Where("customer_id = ?", actor.OrganizationID)
	case data.Reseller:
		return query.Where("reseller_id = ?", actor.OrganizationID)
	case data.Manufacturer:
		return query.Where("manufacturer_id = ? AND inquiry_id <> ''", actor.OrganizationID)
	default:
		return query.Where("1 = 0")
	}
}

func (s Service) Links(ctx context.Context, actor data.User) ([]data.Organization, error) {
	role := actor.Organization.Kind
	if role != data.Customer && role != data.Reseller {
		return nil, ErrForbidden
	}
	return linkedOrganizations(s.db.WithContext(ctx), actor, 0, 0)
}

func linkedOrganizations(db *gorm.DB, actor data.User, id uint64, limit int) ([]data.Organization, error) {
	organizations := make([]data.Organization, 0)
	query := db.Model(&data.Organization{}).Select("pl_organizations.*").Order("pl_organizations.id")
	if actor.Organization.Kind == data.Customer {
		query = query.Joins("JOIN pl_channel_links l ON l.reseller_id = pl_organizations.id").
			Where("l.customer_id = ? AND pl_organizations.kind = ?", actor.OrganizationID, data.Reseller)
	} else {
		query = query.Joins("JOIN pl_channel_links l ON l.customer_id = pl_organizations.id").
			Where("l.reseller_id = ? AND pl_organizations.kind = ?", actor.OrganizationID, data.Customer)
	}
	if id > 0 {
		query = query.Where("pl_organizations.id = ?", id)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&organizations).Error
	return organizations, err
}

func (s Service) List(ctx context.Context, actor data.User, after uint64, limit int) (Page, error) {
	page := Page{Items: []View{}}
	// The rows and events must come from the same snapshot, including across withdrawal.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var projects []channelProject
		if err := visibleProjects(tx, actor).Preload("Customer").Where("id > ?", after).
			Order("id").Limit(limit + 1).Find(&projects).Error; err != nil {
			return err
		}
		if len(projects) > limit {
			projects = projects[:limit]
			cursor := strconv.FormatUint(projects[limit-1].ID, 10)
			page.NextCursor = &cursor
		}
		if len(projects) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(projects))
		for _, project := range projects {
			ids = append(ids, project.ID)
		}
		var events []channelEvent
		if err := tx.Where("project_id IN ?", ids).Order("id").Find(&events).Error; err != nil {
			return err
		}
		byProject := make(map[uint64][]channelEvent)
		for _, event := range events {
			byProject[event.ProjectID] = append(byProject[event.ProjectID], event)
		}
		commercial, err := loadCommercial(tx, ids)
		if err != nil {
			return err
		}
		today := time.Now().In(chinaTime).Format(time.DateOnly)
		for _, project := range projects {
			page.Items = append(page.Items, projectView(project, actor.Organization.Kind, byProject[project.ID], commercial[project.ID], today))
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}

func (s Service) Get(ctx context.Context, actor data.User, id uint64) (View, error) {
	var result View
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project channelProject
		if err := visibleProjects(tx, actor).Preload("Customer").First(&project, id).Error; err != nil {
			return err
		}
		var err error
		result, err = loadView(tx, project, actor.Organization.Kind)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (s Service) Create(ctx context.Context, actor data.User, input CreateInput) (View, error) {
	role := actor.Organization.Kind
	if role != data.Customer && role != data.Reseller {
		return View{}, ErrForbidden
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Requirement = strings.TrimSpace(input.Requirement)
	if !requestIDPattern.MatchString(input.RequestID) || !fits(input.Title, 120) || !fits(input.Requirement, 3000) ||
		!validDate(input.DueDate) || role == data.Customer && input.CustomerID != "" ||
		role == data.Reseller && (input.ResellerID != "" || input.CustomerID == "") {
		return View{}, ErrInvalid
	}
	linkID := input.CustomerID
	if role == data.Customer {
		linkID = input.ResellerID
	}
	var target uint64
	if linkID != "" {
		var err error
		target, err = parseID(linkID)
		if err != nil {
			return View{}, err
		}
	}
	var result View
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, replay, err := claim(tx, actor.OrganizationID, "project:create", input.RequestID, input)
		if err != nil {
			return err
		}
		if replay {
			if request.ProjectID == nil {
				return errors.New("incomplete project request")
			}
			if err := visibleProjects(tx, actor).Where("id = ?", *request.ProjectID).Take(&channelProject{}).Error; err != nil {
				return err
			}
			return json.Unmarshal(request.Result, &result)
		}
		organizations, err := linkedOrganizations(tx, actor, target, 2)
		if err != nil {
			return err
		}
		if len(organizations) != 1 {
			if target > 0 {
				return ErrNotFound
			}
			return ErrLinkRequired
		}
		p := channelProject{
			CustomerID: actor.OrganizationID, ResellerID: organizations[0].ID,
			Title: input.Title, Requirement: input.Requirement, DueDate: input.DueDate, Stage: requested, Revision: 1,
		}
		customer := actor.Organization
		label := "客户提交需求"
		if role == data.Reseller {
			p.CustomerID, p.ResellerID = organizations[0].ID, actor.OrganizationID
			customer, label = organizations[0], "代理商登记需求"
		}
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		p.Customer = customer
		event := eventFor(actor, p, downstream, label, p.Requirement)
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result = projectView(p, role, []channelEvent{event}, commercialState{}, "")
		return saveResult(tx, request, &p.ID, result)
	})
	return result, err
}

func claim(tx *gorm.DB, organizationID uint64, operation, requestID string, input any) (channelRequest, bool, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return channelRequest{}, false, err
	}
	sum := sha256.Sum256(b)
	entry := channelRequest{
		OrganizationID: organizationID, Operation: operation, RequestID: requestID, RequestHash: hex.EncodeToString(sum[:]),
	}
	err = tx.Create(&entry).Error
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return entry, false, err
	}
	var existing channelRequest
	err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Where(
		"organization_id = ? AND operation = ? AND request_id = ?", organizationID, operation, requestID,
	).Take(&existing).Error
	if err != nil {
		return existing, false, err
	}
	if existing.RequestHash != entry.RequestHash {
		return existing, false, ErrIdempotency
	}
	if len(existing.Result) == 0 {
		return existing, false, errors.New("incomplete channel request")
	}
	return existing, true, nil
}

func saveResult(tx *gorm.DB, request channelRequest, projectID *uint64, result any) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return tx.Model(&request).Updates(map[string]any{"project_id": projectID, "result": json.RawMessage(b)}).Error
}

func loadView(tx *gorm.DB, project channelProject, role string) (View, error) {
	commercial, err := loadCommercial(tx, []uint64{project.ID})
	if err != nil {
		return View{}, err
	}
	return viewWithState(tx, project, role, commercial[project.ID], time.Now().In(chinaTime).Format(time.DateOnly))
}

func viewWithState(tx *gorm.DB, project channelProject, role string, commercial commercialState, today string) (View, error) {
	var events []channelEvent
	if err := tx.Where("project_id = ?", project.ID).Order("id").Find(&events).Error; err != nil {
		return View{}, err
	}
	return projectView(project, role, events, commercial, today), nil
}

func eventFor(actor data.User, project channelProject, scope, label, detail string) channelEvent {
	return channelEvent{
		ProjectID: project.ID, ActorUserID: actor.ID, ActorOrgID: actor.OrganizationID, At: time.Now().UTC(),
		Scope: scope, InquiryID: project.InquiryID, Label: label, Detail: detail,
	}
}

func parseID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 || value != strconv.FormatUint(id, 10) {
		return 0, ErrInvalid
	}
	return id, nil
}

func fits(value string, max int) bool {
	return value != "" && utf8.RuneCountInString(value) <= max
}

func validDate(value string) bool {
	if value == "" {
		return true
	}
	date, err := time.Parse(time.DateOnly, value)
	return err == nil && date.Year() >= 1 && date.Format(time.DateOnly) == value
}

func actionPayload(raw []byte, out any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(trimmed))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return ErrInvalid
	}
	return nil
}
