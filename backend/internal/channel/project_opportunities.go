package channel

import (
	"context"
	"database/sql"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

type ProjectOpportunityItem struct {
	ID    uint64 `json:"id,string"`
	Note  string `json:"note"`
	Stage string `json:"stage"`
}

type ProjectOpportunityPage struct {
	Items      []ProjectOpportunityItem `json:"items"`
	NextCursor *string                  `json:"next_cursor"`
}

func (s Service) ProjectOpportunities(ctx context.Context, actor data.User, projectID, before uint64, limit int) (ProjectOpportunityPage, error) {
	if !dealer(actor) {
		return ProjectOpportunityPage{}, ErrForbidden
	}
	if limit < 1 || limit > 50 {
		return ProjectOpportunityPage{}, ErrInvalid
	}
	page := ProjectOpportunityPage{Items: []ProjectOpportunityItem{}}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := visibleProjects(tx, actor).Select("id").Take(&channelProject{}, projectID).Error; err != nil {
			return err
		}
		query := tx.Model(&channelOpportunity{}).
			Select("id", "note", "stage").
			Where("project_id = ? AND organization_id = ?", projectID, actor.OrganizationID)
		if before != 0 {
			query = query.Where("id < ?", before)
		}
		if err := query.Order("id DESC").Limit(limit + 1).Scan(&page.Items).Error; err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			cursor := idString(page.Items[limit-1].ID)
			page.NextCursor = &cursor
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}
