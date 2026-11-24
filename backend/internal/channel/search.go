package channel

import (
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

type ProjectSearchHit struct {
	ID      uint64
	OrderID uint64
	Title   string
}

// SearchProjects uses the regular list scope and projects a title visible to the caller.
func SearchProjects(tx *gorm.DB, actor data.User, terms []string, number uint64, limit int) ([]ProjectSearchHit, error) {
	const projectID = "pl_channel_projects.id"
	title := "pl_channel_projects.title"
	if actor.Organization.Kind == data.Manufacturer {
		title = "CONCAT(pl_channel_projects.product_name, ' · 渠道采购项目')"
	}
	query := visibleProjects(tx, actor).
		Select(projectID + ", " + title + " AS title, COALESCE(commercial.id, 0) AS order_id").
		Joins("LEFT JOIN pl_channel_orders commercial ON commercial.project_id = " + projectID)
	if number > 0 {
		query = query.Where("("+projectID+" = ? OR commercial.id = ?)", number, number)
	} else {
		query = data.MatchSearchTerms(query, "CONCAT_WS(' ', "+projectID+", '项目', commercial.id, IF(commercial.id IS NULL, NULL, '渠道订单'), "+title+")", terms)
	}
	items := make([]ProjectSearchHit, 0, limit)
	err := query.Order(projectID + " DESC").Limit(limit).Scan(&items).Error
	return items, err
}
