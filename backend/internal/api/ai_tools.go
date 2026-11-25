package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/catalog"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

var capabilityLabels = map[string]string{
	"integration": "系统对接服务",
	"connector":   "成品接口 / 连接器",
	"plugin":      "开发工具插件",
	"migration":   "数据迁移",
	"training":    "实施培训",
	"other":       "其他能力",
}

func (s server) executeDealerReadTool(ctx context.Context, actor data.User, name string, args json.RawMessage) (string, string, string, error) {
	switch name {
	case "get_business_overview":
		text, err := s.toolBusinessOverview(ctx, actor)
		return text, "business-overview", "正在读取全局经营统计概览", err
	case "query_customers":
		var p struct {
			Keyword string `json:"keyword"`
			Filter  string `json:"filter"`
			Offset  int    `json:"offset"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryCustomers(ctx, actor, strings.TrimSpace(p.Keyword), strings.TrimSpace(p.Filter), max(0, p.Offset))
		return text, "customers", "正在按需检索客户档案", err
	case "query_renewals_and_orders":
		var p struct {
			Keyword string `json:"keyword"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryRenewalsAndOrders(ctx, actor, strings.TrimSpace(p.Keyword))
		return text, "renewals", "正在查询直购订单与续费回访台账", err
	case "query_projects":
		var p struct {
			ProjectID string `json:"project_id"`
			Keyword   string `json:"keyword"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryProjects(ctx, actor, strings.TrimSpace(p.ProjectID), strings.TrimSpace(p.Keyword))
		return text, "projects", "正在查询协作项目与报价进展", err
	case "query_opportunities_and_partners":
		var p struct {
			Keyword string `json:"keyword"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryOpportunitiesAndPartners(ctx, actor, strings.TrimSpace(p.Keyword))
		return text, "opportunities", "正在查询商机拾遗与团队合作人脉", err
	case "query_service_tickets_and_finance":
		var p struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryTicketsAndFinance(ctx, actor, strings.TrimSpace(p.Status))
		return text, "service-tickets", "正在查询服务工单与财务申请记录", err
	case "query_product_catalog":
		var p struct {
			Keyword string `json:"keyword"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolQueryProductCatalog(ctx, actor, strings.TrimSpace(p.Keyword))
		return text, "catalog", "正在查询已发布软件目录与产品知识库", err
	default:
		return "未知查询工具：" + name, "", "", nil
	}
}

func (s server) toolBusinessOverview(ctx context.Context, actor data.User) (string, error) {
	var b strings.Builder
	today := time.Now().In(chinaTime).Format(time.DateOnly)
	fmt.Fprintf(&b, "统计日期：%s | 组织：%s\n", today, actor.Organization.Name)

	if s.db != nil {
		dealer := actor.OrganizationID
		var totals operationsTotals
		if err := s.db.WithContext(ctx).Raw(`
			SELECT (SELECT COUNT(*) FROM (`+dealerCustomerOrganizations+`) known)
				+ (SELECT COUNT(*) FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL) AS customers,
				COUNT(*) AS orders, COUNT(CASE WHEN source = 'promotion' THEN 1 END) AS promotion_orders,
				COUNT(CASE WHEN renewal_due_on BETWEEN ? AND DATE_ADD(?, INTERVAL 30 DAY) THEN 1 END) AS renewals_30_days,
				COALESCE(SUM(CASE WHEN status = 'settlement' THEN `+retailCommissionCents+` ELSE 0 END), 0) AS pending_commission_cents,
				(SELECT COUNT(*) FROM pl_channel_service_tickets WHERE reseller_id = ? AND status <> 'closed') AS open_tickets
			FROM pl_retail_orders WHERE reseller_id = ?`,
			dealer, dealer, dealer, today, today, dealer, dealer).Scan(&totals).Error; err == nil {
			fmt.Fprintf(&b, "客户档案总数：%d 家 | 直购订单：%d 笔（其中推广码订单 %d 笔） | 待结算佣金：%.2f 元 | 30天内到期续费：%d 笔 | 未关闭工单：%d 单\n",
				totals.Customers, totals.Orders, totals.PromotionOrders, float64(totals.PendingCommissionCents)/100.0, totals.Renewals30Days, totals.OpenTickets)
		}
	}
	if page, err := s.channel.List(ctx, actor, 0, 50); err == nil {
		todoCount := 0
		for _, p := range page.Items {
			if p.IsTodo {
				todoCount++
			}
		}
		fmt.Fprintf(&b, "协作项目总数：%d 个（其中待我处理 %d 个）\n", len(page.Items), todoCount)
	}
	if desk, err := s.channel.Desk(ctx, actor); err == nil {
		fmt.Fprintf(&b, "团队附加商机：%d 条 | 已登记合作人脉伙伴：%d 位 | 待跟进回访提醒：%d 项\n",
			len(desk.Items), len(desk.Partners), len(desk.Tasks))
	}
	return b.String(), nil
}

func (s server) toolQueryCustomers(ctx context.Context, actor data.User, keyword, filter string, offset int) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	dealer := actor.OrganizationID
	allRows := make([]operationsCustomer, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT * FROM (
			SELECT CONCAT('o', org.id) AS id, org.name, COALESCE(profile.contact, '') AS contact,
				COALESCE(profile.note, '') AS note, link.customer_id IS NOT NULL AS linked, FALSE AS prospect
			FROM (`+dealerCustomerOrganizations+`) known
			JOIN pl_organizations org ON org.id = known.customer_id
			LEFT JOIN pl_channel_links link ON link.customer_id = org.id AND link.reseller_id = ?
			LEFT JOIN pl_dealer_customer_profiles profile ON profile.customer_id = org.id AND profile.reseller_id = ?
			UNION ALL
			SELECT CONCAT('p', id), prospect_name, contact, note, FALSE, TRUE
			FROM pl_dealer_customer_profiles WHERE reseller_id = ? AND customer_id IS NULL
		) customers ORDER BY id`,
		dealer, dealer, dealer, dealer, dealer).Scan(&allRows).Error
	if err != nil {
		return "", err
	}

	kw := strings.ToLower(keyword)
	filtered := make([]operationsCustomer, 0, len(allRows))
	for _, row := range allRows {
		if filter == "linked" && !row.Linked {
			continue
		}
		if filter == "prospect" && !row.Prospect {
			continue
		}
		if kw != "" {
			haystack := strings.ToLower(row.Name + " " + row.Contact + " " + row.Note)
			if !strings.Contains(haystack, kw) {
				continue
			}
		}
		filtered = append(filtered, row)
	}

	total := len(filtered)
	if total == 0 {
		return "未找到符合条件的客户档案。", nil
	}
	const pageSize = 50
	if offset >= total {
		offset = 0
	}
	end := min(total, offset+pageSize)
	slice := filtered[offset:end]

	var b strings.Builder
	linkedTotal := 0
	prospectTotal := 0
	for _, c := range filtered {
		if c.Linked {
			linkedTotal++
		} else if c.Prospect {
			prospectTotal++
		}
	}
	fmt.Fprintf(&b, "匹配客户共 %d 家（已关联签约 %d 家，潜在跟进 %d 家），当前展示第 %d–%d 家：\n",
		total, linkedTotal, prospectTotal, offset+1, end)
	for _, cust := range slice {
		tag := "直购客户"
		if cust.Linked {
			tag = "已关联签约客户"
		} else if cust.Prospect {
			tag = "潜在跟进客户"
		}
		fmt.Fprintf(&b, "- %s [%s] | 联系人：%s | 备注：%s\n",
			cust.Name, tag, fallbackDash(cust.Contact), fallbackDash(cust.Note))
	}
	if end < total {
		fmt.Fprintf(&b, "（尚有 %d 家未展示，可传 offset=%d 继续查询）\n", total-end, end)
	}
	return b.String(), nil
}

func (s server) toolQueryRenewalsAndOrders(ctx context.Context, actor data.User, keyword string) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	dealer := actor.OrganizationID
	var b strings.Builder
	kw := strings.ToLower(keyword)

	renewals := make([]renewalEntry, 0)
	_ = s.db.WithContext(ctx).Raw(`
		SELECT orders.id, orders.id AS order_id, client.name AS customer, orders.product_name, orders.product_version,
			orders.source, orders.quantity, orders.paid_at, DATE_FORMAT(orders.renewal_due_on, '%Y-%m-%d') AS due_at,
			COALESCE(followup.note, '') AS note, followup.contacted_at
		FROM pl_retail_orders orders JOIN pl_organizations client ON client.id = orders.customer_id
		LEFT JOIN pl_dealer_renewal_notes followup ON orders.id = followup.order_id
		WHERE orders.reseller_id = ? AND orders.renewal_due_on IS NOT NULL
		ORDER BY orders.renewal_due_on ASC LIMIT 50`, dealer).Scan(&renewals).Error

	b.WriteString("【年度授权与续费回访台账】\n")
	matchedRenewals := 0
	for _, r := range renewals {
		if kw != "" && !strings.Contains(strings.ToLower(fmt.Sprintf("%d %s %s %s", r.OrderID, r.Customer, r.ProductName, r.Note)), kw) {
			continue
		}
		matchedRenewals++
		fmt.Fprintf(&b, "- 订单#%d | 客户：%s | 产品：%s %s × %d套 | 购买日：%s | 到期日：%s | 回访备注：%s\n",
			r.OrderID, r.Customer, r.ProductName, r.ProductVersion, r.Quantity,
			r.PaidAt.In(chinaTime).Format(time.DateOnly), r.DueAt, fallbackDash(r.Note))
	}
	if matchedRenewals == 0 {
		b.WriteString("- 暂无匹配的年度续费订单记录。\n")
	}

	orders := make([]retailOrderRow, 0)
	_ = s.db.WithContext(ctx).Table("pl_retail_orders AS orders").
		Select("orders.*, maker.name AS manufacturer, partner.name AS reseller, client.name AS customer").
		Joins("JOIN pl_organizations AS maker ON maker.id = orders.manufacturer_id").
		Joins("JOIN pl_organizations AS partner ON partner.id = orders.reseller_id").
		Joins("JOIN pl_organizations AS client ON client.id = orders.customer_id").
		Where("orders.reseller_id = ?", dealer).
		Order("orders.id DESC").Limit(40).Scan(&orders).Error

	b.WriteString("\n【直购订单与佣金结算明细】\n")
	matchedOrders := 0
	for _, o := range orders {
		if kw != "" && !strings.Contains(strings.ToLower(fmt.Sprintf("%d %s %s %s", o.ID, o.Customer, o.ProductName, o.Manufacturer)), kw) {
			continue
		}
		matchedOrders++
		totalYuan := float64(o.UnitCents*uint64(o.Quantity)) / 100.0
		commYuan := float64((o.UnitCents*uint64(o.Quantity)*uint64(o.CommissionBPS)+5000)/10000) / 100.0
		fmt.Fprintf(&b, "- 直购单#%d | 客户：%s | 产品：%s × %d | 厂商：%s | 订单总额：%.2f元 | 应计佣金：%.2f元 | 状态：%s\n",
			o.ID, o.Customer, o.ProductName, o.Quantity, o.Manufacturer, totalYuan, commYuan, o.Status)
	}
	if matchedOrders == 0 {
		b.WriteString("- 暂无匹配的直购订单。\n")
	}
	return b.String(), nil
}

func (s server) toolQueryProjects(ctx context.Context, actor data.User, projectID, keyword string) (string, error) {
	if projectID != "" {
		cleanID := strings.TrimPrefix(strings.TrimSpace(projectID), "#")
		idNum, err := strconv.ParseUint(cleanID, 10, 64)
		if err != nil || idNum == 0 {
			return "项目编号格式无效：" + projectID, nil
		}
		view, err := s.channel.Get(ctx, actor, idNum)
		if err != nil {
			return "未找到项目 #" + cleanID + " 或当前账号无权查看。", nil
		}
		var b strings.Builder
		custName := ""
		if view.CustomerName != nil {
			custName = *view.CustomerName
		}
		reqText := ""
		if view.Requirement != nil {
			reqText = *view.Requirement
		}
		fmt.Fprintf(&b, "项目 #%s《%s》详情：\n- 客户：%s | 当前阶段：%s | 待我处理：%v | 下一步：%s | 期望日期：%s\n- 客户采购需求：%s\n",
			view.ID, view.Title, fallbackDash(custName), view.StageLabel, view.IsTodo, view.NextStep, fallbackDash(view.DueDate), fallbackDash(reqText))
		if view.Inquiry != nil {
			fmt.Fprintf(&b, "- 厂商询价单(%s)：%s（能力缺口说明：%s）\n", view.Inquiry.ID, view.Inquiry.Summary, fallbackDash(view.Inquiry.GapNote))
		}
		if view.Channel != nil && len(*view.Channel) > 0 {
			latest := (*view.Channel)[len(*view.Channel)-1]
			fmt.Fprintf(&b, "- 最新渠道底价(%s)：合计 %v 元，有效期至 %s，交付周期 %d 天\n", latest.ID, latest.Total, latest.ValidUntil, latest.DeliveryDays)
		}
		if view.Sales != nil && len(*view.Sales) > 0 {
			latest := (*view.Sales)[len(*view.Sales)-1]
			fmt.Fprintf(&b, "- 最新对客报价(%s)：合计 %v 元，状态 %s，有效期至 %s\n", latest.ID, latest.Total, latest.Status, latest.ValidUntil)
		}
		if view.Order != nil {
			fmt.Fprintf(&b, "- 正式订单号：%s\n", view.Order.ID)
		}
		return b.String(), nil
	}

	page, err := s.channel.List(ctx, actor, 0, 50)
	if err != nil {
		return "", err
	}
	kw := strings.ToLower(keyword)
	var b strings.Builder
	count := 0
	for _, p := range page.Items {
		custName := ""
		if p.CustomerName != nil {
			custName = *p.CustomerName
		}
		if kw != "" && !strings.Contains(strings.ToLower(p.ID+" "+p.Title+" "+custName+" "+p.StageLabel), kw) {
			continue
		}
		count++
		todoTag := ""
		if p.IsTodo {
			todoTag = "【待我处理】"
		}
		fmt.Fprintf(&b, "- 项目#%s《%s》| 客户：%s | 阶段：%s %s | 下一步：%s | 可执行动作：%s\n",
			p.ID, p.Title, fallbackDash(custName), p.StageLabel, todoTag, p.NextStep, strings.Join(p.Actions, ","))
	}
	if count == 0 {
		return "当前暂无符合条件的协作项目。", nil
	}
	return fmt.Sprintf("协作项目共匹配 %d 个：\n%s", count, b.String()), nil
}

func (s server) toolQueryOpportunitiesAndPartners(ctx context.Context, actor data.User, keyword string) (string, error) {
	desk, err := s.channel.Desk(ctx, actor)
	if err != nil {
		return "", err
	}
	kw := strings.ToLower(keyword)
	var b strings.Builder

	b.WriteString("【团队合作伙伴人脉库】\n")
	pCount := 0
	for _, partner := range desk.Partners {
		line := fmt.Sprintf("%s %s %s %s %s %s", partner.Name, partner.Owner, partner.Category, partner.Source, partner.Target, partner.Evidence)
		if kw != "" && !strings.Contains(strings.ToLower(line), kw) {
			continue
		}
		pCount++
		capLabel := capabilityLabels[partner.Category]
		if capLabel == "" {
			capLabel = partner.Category
		}
		sysScope := strings.ToUpper(strings.Trim(partner.Source+" -> "+partner.Target, " ->"))
		fmt.Fprintf(&b, "- 伙伴：%s | 内部引荐人：%s | 擅长能力：%s（系统范围：%s）| 历史合作依据：%s\n",
			partner.Name, partner.Owner, capLabel, fallbackDash(sysScope), fallbackDash(partner.Evidence))
	}
	if pCount == 0 {
		b.WriteString("- 暂无匹配的登记合作伙伴。\n")
	}

	b.WriteString("\n【商机拾遗（团队附加商机台账）】\n")
	oCount := 0
	for _, item := range desk.Items {
		line := fmt.Sprintf("%s %s %s %s %s", item.ProjectTitle, item.Note, item.Category, item.Source, item.Target)
		if kw != "" && !strings.Contains(strings.ToLower(line), kw) {
			continue
		}
		oCount++
		fmt.Fprintf(&b, "- 商机#%s（关联主项目：%s）| 需求：%s | 状态：%s | 类型：%s (%s -> %s)\n",
			item.ID, item.ProjectTitle, item.Note, item.Stage, capabilityLabels[item.Category], strings.ToUpper(item.Source), strings.ToUpper(item.Target))
	}
	if oCount == 0 {
		b.WriteString("- 暂无匹配的附加商机记录。\n")
	}

	if len(desk.Tasks) > 0 {
		b.WriteString("\n【近期商机回访待办】\n")
		for _, task := range desk.Tasks {
			fmt.Fprintf(&b, "- 商机#%s（%s）：%s，建议动作：%s（日期：%s）\n",
				task.OpportunityID, task.ProjectTitle, task.Reason, task.Action, task.Date)
		}
	}
	return b.String(), nil
}

func (s server) toolQueryTicketsAndFinance(ctx context.Context, actor data.User, statusFilter string) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	dealer := actor.OrganizationID
	var b strings.Builder

	tickets := make([]channelServiceTicket, 0)
	q := s.db.WithContext(ctx).Where("reseller_id = ?", dealer)
	if statusFilter != "" && statusFilter != "all" {
		q = q.Where("status = ?", statusFilter)
	}
	_ = q.Order("id DESC").Limit(25).Find(&tickets).Error

	if len(tickets) > 0 {
		ids := make([]uint64, 0, len(tickets))
		pos := make(map[uint64]int, len(tickets))
		for i := range tickets {
			ids = append(ids, tickets[i].ID)
			pos[tickets[i].ID] = i
		}
		var actions []channelTicketAction
		_ = s.db.WithContext(ctx).Where("ticket_id IN ? AND action <> 'close'", ids).Order("ticket_id, revision").Find(&actions).Error
		for _, a := range actions {
			idx := pos[a.TicketID]
			tickets[idx].Replies = append(tickets[idx].Replies, a)
		}
	}

	b.WriteString("【渠道售后服务工单明细】\n")
	if len(tickets) == 0 {
		b.WriteString("- 当前无符合状态条件的服务工单。\n")
	}
	for _, t := range tickets {
		orderTag := "未关联订单"
		if t.OrderID != nil {
			orderTag = fmt.Sprintf("关联订单#%d", *t.OrderID)
		}
		fmt.Fprintf(&b, "- 工单#%d《%s》| 分类：%s | 状态：%s | %s\n  问题描述：%s\n",
			t.ID, t.Subject, t.Category, t.Status, orderTag, t.Detail)
		for _, rep := range t.Replies {
			fmt.Fprintf(&b, "  [%s回复]：%s\n", rep.Role, rep.Text)
		}
	}

	finances := make([]channelFinanceRequest, 0)
	_ = s.db.WithContext(ctx).Where("reseller_id = ?", dealer).Order("id DESC").Limit(20).Find(&finances).Error
	b.WriteString("\n【渠道财务申请记录（预存 / 开票 / 服务费报销）】\n")
	if len(finances) == 0 {
		b.WriteString("- 当前暂无预存、开票或服务费报销申请记录。\n")
	}
	for _, f := range finances {
		fmt.Fprintf(&b, "- 申请#%d | 类型：%s | 状态：%s | 金额：%.2f元 | 抬头/说明：%s | 结果凭证：%s\n",
			f.ID, f.Kind, f.Status, float64(f.AmountCents)/100.0, fallbackDash(f.Title), fallbackDash(f.ResultReference))
	}
	return b.String(), nil
}

func (s server) toolQueryProductCatalog(ctx context.Context, actor data.User, keyword string) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	type catalogRow struct {
		data.Product      `gorm:"embedded"`
		catalog.Knowledge `gorm:"embedded"`
		UnitCents         *uint64 `gorm:"column:unit_cents"`
		CommissionBPS     *uint32 `gorm:"column:commission_bps"`
		Annual            *bool   `gorm:"column:annual"`
	}
	var rows []catalogRow
	err := catalog.Visible(s.db.WithContext(ctx), actor).
		Where("pl_products.published = ?", true).
		Select("pl_products.*, k.category, k.modules, k.deployment_modes, k.license_minimum, k.license_maximum, k.license_note, k.sources, o.unit_cents, o.commission_bps, o.annual").
		Joins("LEFT JOIN pl_product_knowledge_profiles k ON k.product_id = pl_products.id").
		Joins("LEFT JOIN pl_retail_offers o ON o.product_id = pl_products.id AND o.active = TRUE").
		Order("pl_products.id").Limit(40).Find(&rows).Error
	if err != nil {
		return "", err
	}

	kw := strings.ToLower(keyword)
	var b strings.Builder
	count := 0
	for _, r := range rows {
		modStr := strings.Join(r.Modules, "、")
		depStr := strings.Join(r.DeploymentModes, "/")
		if kw != "" && !strings.Contains(strings.ToLower(r.Name+" "+r.Version+" "+r.Description+" "+modStr+" "+r.Category), kw) {
			continue
		}
		count++
		seatRange := "未限制"
		if r.License.Minimum != nil || r.License.Maximum != nil {
			minStr, maxStr := "不限", "不限"
			if r.License.Minimum != nil {
				minStr = strconv.Itoa(*r.License.Minimum)
			}
			if r.License.Maximum != nil {
				maxStr = strconv.Itoa(*r.License.Maximum)
			}
			seatRange = fmt.Sprintf("%s–%s 席", minStr, maxStr)
		}
		retailInfo := "暂未开放直购"
		if r.UnitCents != nil {
			cycle := "买断"
			if r.Annual != nil && *r.Annual {
				cycle = "年度订阅"
			}
			bps := uint32(0)
			if r.CommissionBPS != nil {
				bps = *r.CommissionBPS
			}
			retailInfo = fmt.Sprintf("直购价 %.2f 元（%s，代理商分佣 %.1f%%）", float64(*r.UnitCents)/100.0, cycle, float64(bps)/100.0)
		}
		srcInfo := ""
		if len(r.Sources) > 0 {
			srcInfo = fmt.Sprintf("%s（%s）：%s", r.Sources[0].Title, r.Sources[0].Reference, r.Sources[0].Excerpt)
		}
		fmt.Fprintf(&b, "- 产品#%d《%s %s》| 类别：%s | 模块：%s | 部署方式：%s | 授权席位范围：%s | %s\n  说明：%s | 资料依据：%s\n",
			r.ID, r.Name, r.Version, fallbackDash(r.Category), fallbackDash(modStr), fallbackDash(depStr), seatRange, retailInfo,
			fallbackDash(r.Description), fallbackDash(srcInfo))
	}
	if count == 0 {
		return "当前未检索到符合条件的已发布软件产品。", nil
	}
	return fmt.Sprintf("平台已发布软件产品匹配 %d 款：\n%s", count, b.String()), nil
}
