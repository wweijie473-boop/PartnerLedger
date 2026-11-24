package channel

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProjectListItem struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	CustomerName *string `json:"customerName,omitempty"`
	DueDate      string  `json:"dueDate,omitempty"`
	Stage        string  `json:"stage"`
	StageLabel   string  `json:"stageLabel"`
	IsTodo       bool    `json:"isTodo"`
	NextStep     string  `json:"nextStep"`
}

type ProjectWorkspaceQuery struct {
	Scope, Stage, Search, After string
	Limit                       int
}

type ProjectWorkspacePage struct {
	Items      []ProjectListItem `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	Counts     ProjectCounts     `json:"counts"`
	Stages     []ProjectStage    `json:"stages"`
}

type ProjectCounts struct {
	All      int64 `json:"all"`
	Mine     int64 `json:"mine"`
	Partners int64 `json:"partners"`
}

type ProjectStage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

const projectDateOrder = "COALESCE(NULLIF(pl_channel_projects.due_date, ''), '9999')"

func (s Service) Workspace(ctx context.Context, actor data.User, filter ProjectWorkspaceQuery) (ProjectWorkspacePage, error) {
	page := ProjectWorkspacePage{Items: []ProjectListItem{}, Stages: []ProjectStage{}}
	role := actor.Organization.Kind
	dateOrder := filter.Scope == "mine" && role != data.Manufacturer
	filter.Search = strings.TrimSpace(filter.Search)
	afterDate, afterID, err := workspaceCursor(filter.After, dateOrder)
	if err != nil || filter.Limit < 1 || filter.Limit > 100 || utf8.RuneCountInString(filter.Search) > 120 ||
		(filter.Scope != "all" && filter.Scope != "mine" && filter.Scope != "partners") ||
		(filter.Stage != "" && stageLabels[filter.Stage] == "") {
		return page, ErrInvalid
	}
	// Counts, cursor rows and the quote state used to label them share a snapshot.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		today := time.Now().In(chinaTime).Format(time.DateOnly)
		todo := projectTodoPredicate(role, today)
		if err := page.summarize(visibleProjects(tx, actor), todo, role); err != nil {
			return err
		}
		query := visibleProjects(tx, actor)
		switch filter.Scope {
		case "mine":
			query = query.Where(todo)
		case "partners":
			query = query.Where("NOT (?) AND pl_channel_projects.stage <> ?", todo, completed)
		}
		if filter.Stage != "" {
			var matching []string
			for stage := range stageLabels {
				if visibleStage(stage, role) == filter.Stage {
					matching = append(matching, stage)
				}
			}
			query = query.Where("pl_channel_projects.stage IN ?", matching)
		}
		if filter.Search != "" {
			query = searchWorkspaceProjects(query, role, filter.Search)
		}
		if dateOrder {
			query = query.Order(projectDateOrder)
			if afterID > 0 {
				query = query.Where("("+projectDateOrder+", pl_channel_projects.id) > (?, ?)", afterDate, afterID)
			}
		} else if afterID > 0 {
			query = query.Where("pl_channel_projects.id > ?", afterID)
		}
		query = query.Order("pl_channel_projects.id").Limit(filter.Limit + 1)
		if role != data.Manufacturer {
			query = query.Preload("Customer", func(db *gorm.DB) *gorm.DB { return db.Select("id", "name") })
		}
		var projects []channelProject
		if err := query.Select("pl_channel_projects.id", "pl_channel_projects.title", "pl_channel_projects.customer_id",
			"pl_channel_projects.product_name", "pl_channel_projects.due_date", "pl_channel_projects.stage",
			"pl_channel_projects.processed", "pl_channel_projects.gap_note").Find(&projects).Error; err != nil {
			return err
		}
		if len(projects) > filter.Limit {
			projects = projects[:filter.Limit]
			last := projects[len(projects)-1]
			cursor := idString(last.ID)
			if dateOrder {
				cursor = last.DueDate + ":" + cursor
			}
			page.NextCursor = &cursor
		}
		if len(projects) == 0 {
			return nil
		}
		state, err := loadWorkspaceQuoteStatus(tx, projects)
		if err != nil {
			return err
		}
		for _, project := range projects {
			item, _ := projectListItem(project, role, state[project.ID], today)
			page.Items = append(page.Items, item)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}

func workspaceCursor(value string, dateOrder bool) (date string, id uint64, err error) {
	if value == "" {
		return "", 0, nil
	}
	if dateOrder {
		var found bool
		date, value, found = strings.Cut(value, ":")
		if !found || !validDate(date) {
			return "", 0, ErrInvalid
		}
		if date == "" {
			date = "9999"
		}
	}
	id, err = strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return "", 0, ErrInvalid
	}
	return date, id, nil
}

func projectTodoPredicate(role, today string) clause.Expr {
	var owned []string
	for stage, owner := range stageOwners {
		if owner == role {
			owned = append(owned, stage)
		}
	}
	sort.Strings(owned)
	if role != data.Manufacturer {
		return clause.Expr{SQL: "pl_channel_projects.stage IN ?", Vars: []any{owned}}
	}
	// A manufacturer can also have a renewal task after the channel quote expires.
	return clause.Expr{SQL: `(pl_channel_projects.stage IN ? OR (pl_channel_projects.stage IN ? AND EXISTS (
		SELECT 1 FROM pl_channel_quotes q WHERE q.id = (
			SELECT latest.id FROM pl_channel_quotes latest WHERE latest.project_id = pl_channel_projects.id
				AND latest.kind = ? ORDER BY latest.version DESC LIMIT 1
		) AND q.status IN ? AND q.valid_until < ?
	)))`, Vars: []any{owned, []string{channelQuoted, salesQuoted, salesRevision, confirmed}, channelKind, []string{activeQuote, confirmed}, today}}
}

func searchWorkspaceProjects(query *gorm.DB, role, keyword string) *gorm.DB {
	terms := []string{keyword}
	if role == data.Manufacturer {
		return data.MatchSearchTerms(query, "CONCAT_WS(' ', pl_channel_projects.id, CONCAT(pl_channel_projects.product_name, ' · 渠道采购项目'))", terms)
	}
	query = query.Joins("JOIN pl_organizations customer ON customer.id = pl_channel_projects.customer_id")
	return data.MatchSearchTerms(query, "CONCAT_WS(' ', pl_channel_projects.id, pl_channel_projects.title, customer.name)", terms)
}

func (page *ProjectWorkspacePage) summarize(query *gorm.DB, todo clause.Expr, role string) error {
	var groups []struct {
		Stage string
		Todo  bool
		Total int64
	}
	if err := query.Select("pl_channel_projects.stage, CASE WHEN ? THEN 1 ELSE 0 END AS todo, COUNT(*) AS total", todo).
		Group("pl_channel_projects.stage, todo").Scan(&groups).Error; err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, group := range groups {
		page.Counts.All += group.Total
		if group.Todo {
			page.Counts.Mine += group.Total
		} else if group.Stage != completed {
			page.Counts.Partners += group.Total
		}
		stage := visibleStage(group.Stage, role)
		if !seen[stage] {
			seen[stage] = true
			page.Stages = append(page.Stages, ProjectStage{ID: stage, Label: stageLabels[stage]})
		}
	}
	sort.Slice(page.Stages, func(i, j int) bool { return page.Stages[i].ID < page.Stages[j].ID })
	return nil
}

func loadWorkspaceQuoteStatus(tx *gorm.DB, projects []channelProject) (map[uint64]commercialState, error) {
	ids := make([]uint64, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	latest := tx.Model(&channelQuote{}).Select("project_id, kind, MAX(version) AS version").
		Where("project_id IN ?", ids).Group("project_id, kind")
	var quotes []channelQuote
	if err := tx.Select("pl_channel_quotes.project_id", "pl_channel_quotes.kind", "pl_channel_quotes.status", "pl_channel_quotes.valid_until").
		Joins("JOIN (?) latest ON latest.project_id = pl_channel_quotes.project_id AND latest.kind = pl_channel_quotes.kind AND latest.version = pl_channel_quotes.version", latest).
		Find(&quotes).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]commercialState)
	for _, quote := range quotes {
		state := result[quote.ProjectID]
		if quote.Kind == channelKind {
			state.channelQuotes = []channelQuote{quote}
		} else {
			state.salesQuotes = []channelQuote{quote}
		}
		result[quote.ProjectID] = state
	}
	return result, nil
}
