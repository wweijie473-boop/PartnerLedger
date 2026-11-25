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

func (s server) executeVendorReadTool(ctx context.Context, actor data.User, name string, args json.RawMessage) (string, string, string, error) {
	switch name {
	case "vendor_overview":
		text, err := s.toolVendorOverview(ctx, actor)
		return text, "projects", "正在读取厂商渠道运营总览", err
	case "vendor_query_inquiries":
		var p struct {
			ProjectID string `json:"project_id"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolVendorInquiries(ctx, actor, strings.TrimSpace(p.ProjectID))
		return text, "projects", "正在查询渠道询价与供货进展", err
	case "vendor_query_products":
		text, err := s.toolVendorProducts(ctx, actor)
		return text, "catalog", "正在读取本厂软件目录与知识库", err
	case "vendor_query_orders_and_settlements":
		var p struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolVendorOrders(ctx, actor, strings.TrimSpace(p.Status))
		return text, "manufacturer-settlements", "正在查询直购收款与佣金结算明细", err
	case "vendor_query_finance_and_tickets":
		text, err := s.toolVendorFinanceAndTickets(ctx, actor)
		return text, "vendor-tickets", "正在查询渠道财务申请与售后工单", err
	default:
		return "未知查询工具：" + name, "", "", nil
	}
}

func (s server) executeCustomerReadTool(ctx context.Context, actor data.User, name string, args json.RawMessage) (string, string, string, error) {
	switch name {
	case "customer_query_projects":
		var p struct {
			ProjectID string `json:"project_id"`
		}
		_ = json.Unmarshal(args, &p)
		text, err := s.toolCustomerProjects(ctx, actor, strings.TrimSpace(p.ProjectID))
		return text, "projects", "正在查询本企业采购项目与对客报价", err
	case "customer_query_orders_and_dealers":
		text, err := s.toolCustomerOrdersAndDealers(ctx, actor)
		return text, "retail-orders", "正在查询直购订单与合作代理商", err
	default:
		return "未知查询工具：" + name, "", "", nil
	}
}

func (s server) toolVendorOverview(ctx context.Context, actor data.User) (string, error) {
	var b strings.Builder
	today := time.Now().In(chinaTime).Format(time.DateOnly)
	fmt.Fprintf(&b, "日期：%s | 软件厂商：%s\n", today, actor.Organization.Name)

	if list, err := s.channel.List(ctx, actor, 0, 50); err == nil {
		todo := 0
		for _, item := range list.Items {
			if item.IsTodo {
				todo++
			}
		}
		fmt.Fprintf(&b, "渠道询价项目总数：%d 个（其中待我厂回复报价或供货 %d 个）\n", len(list.Items), todo)
	}
	if s.db != nil {
		maker := actor.OrganizationID
		var prodCount, pubCount, paymentOrders, settlementOrders, pendingFinance, openTickets int64
		var settlementCents uint64
		_ = s.db.WithContext(ctx).Model(&data.Product{}).Where("manufacturer_id = ?", maker).Count(&prodCount).Error
		_ = s.db.WithContext(ctx).Model(&data.Product{}).Where("manufacturer_id = ? AND published = TRUE", maker).Count(&pubCount).Error
		_ = s.db.WithContext(ctx).Table("pl_retail_orders").Where("manufacturer_id = ? AND status = 'payment'", maker).Count(&paymentOrders).Error
		_ = s.db.WithContext(ctx).Table("pl_retail_orders").Where("manufacturer_id = ? AND status = 'settlement'", maker).Count(&settlementOrders).Error
		_ = s.db.WithContext(ctx).Table("pl_retail_orders").
			Where("manufacturer_id = ? AND status = 'settlement'", maker).
			Select("COALESCE(SUM(" + retailCommissionCents + "), 0)").Scan(&settlementCents).Error
		_ = s.db.WithContext(ctx).Table("pl_channel_finance_requests").
			Where("manufacturer_id = ? AND status IN ('pending', 'requested')", maker).Count(&pendingFinance).Error
		_ = s.db.WithContext(ctx).Table("pl_channel_service_tickets").
			Where("manufacturer_id = ? AND status = 'open'", maker).Count(&openTickets).Error

		fmt.Fprintf(&b, "本厂产品：%d 款（已发布 %d 款） | 待确认收款直购单：%d 笔 | 待结算佣金：%d 笔（共 %.2f 元） | 待办财务申请：%d 笔 | 待回复渠道工单：%d 张\n",
			prodCount, pubCount, paymentOrders, settlementOrders, float64(settlementCents)/100.0, pendingFinance, openTickets)
	}
	return b.String(), nil
}

func (s server) toolVendorInquiries(ctx context.Context, actor data.User, projectID string) (string, error) {
	if projectID != "" {
		clean := strings.TrimPrefix(projectID, "#")
		idNum, err := strconv.ParseUint(clean, 10, 64)
		if err != nil || idNum == 0 {
			return "无效项目编号：" + projectID, nil
		}
		view, err := s.channel.Get(ctx, actor, idNum)
		if err != nil {
			return "未找到该渠道询价项目或无权访问。", nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "渠道项目 #%s《%s》| 阶段：%s | 待我厂处理：%v | 下一步：%s | 可执行动作：%s\n",
			view.ID, view.Title, view.StageLabel, view.IsTodo, view.NextStep, strings.Join(view.Actions, ","))
		if view.Inquiry != nil {
			fmt.Fprintf(&b, "- 询价摘要（%s）：%s | 能力缺口记录：%s\n", view.Inquiry.ID, view.Inquiry.Summary, fallbackDash(view.Inquiry.GapNote))
		}
		if view.Channel != nil && len(*view.Channel) > 0 {
			latest := (*view.Channel)[len(*view.Channel)-1]
			fmt.Fprintf(&b, "- 最新渠道底价（%s）：合计 %v 元，交付天数 %d 天，有效期至 %s\n",
				latest.ID, latest.Total, latest.DeliveryDays, latest.ValidUntil)
		}
		if view.Order != nil {
			fmt.Fprintf(&b, "- 已生成采购订单：%s | 供货说明：%s\n", view.Order.ID, fallbackDash(view.SupplyNote))
		}
		return b.String(), nil
	}

	page, err := s.channel.List(ctx, actor, 0, 50)
	if err != nil {
		return "", err
	}
	if len(page.Items) == 0 {
		return "当前暂无收到代理商提交的渠道询价。", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共收到 %d 个渠道询价/采购项目：\n", len(page.Items))
	for _, item := range page.Items {
		todo := ""
		if item.IsTodo {
			todo = "【待我厂办理】"
		}
		summary := ""
		if item.Inquiry != nil {
			summary = item.Inquiry.Summary
		}
		fmt.Fprintf(&b, "- 项目#%s《%s》| 阶段：%s %s | 可办动作：%s | 询价摘要：%s\n",
			item.ID, item.Title, item.StageLabel, todo, strings.Join(item.Actions, ","), fallbackDash(summary))
	}
	return b.String(), nil
}

func (s server) toolVendorProducts(ctx context.Context, actor data.User) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	type row struct {
		data.Product      `gorm:"embedded"`
		catalog.Knowledge `gorm:"embedded"`
		UnitCents         *uint64 `gorm:"column:unit_cents"`
		CommissionBPS     *uint32 `gorm:"column:commission_bps"`
		OfferActive       *bool   `gorm:"column:offer_active"`
		Annual            *bool   `gorm:"column:annual"`
	}
	var rows []row
	err := s.db.WithContext(ctx).Table("pl_products").
		Select("pl_products.*, k.category, k.modules, k.deployment_modes, k.license_minimum, k.license_maximum, k.license_note, k.sources, o.unit_cents, o.commission_bps, o.active AS offer_active, o.annual").
		Joins("LEFT JOIN pl_product_knowledge_profiles k ON k.product_id = pl_products.id").
		Joins("LEFT JOIN pl_retail_offers o ON o.product_id = pl_products.id").
		Where("pl_products.manufacturer_id = ?", actor.OrganizationID).
		Order("pl_products.id").Find(&rows).Error
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "本厂暂未创建任何软件产品。", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "本厂名下共 %d 款软件产品：\n", len(rows))
	for _, r := range rows {
		pub := "未发布"
		if r.Published {
			pub = "已发布"
		}
		pricing := "未配置直购定价"
		if r.UnitCents != nil {
			state := "停用"
			if r.OfferActive != nil && *r.OfferActive {
				state = "开放直购"
			}
			bps := uint32(0)
			if r.CommissionBPS != nil {
				bps = *r.CommissionBPS
			}
			pricing = fmt.Sprintf("%s，单价 %.2f 元，代理分佣 %.1f%%", state, float64(*r.UnitCents)/100.0, float64(bps)/100.0)
		}
		minS, maxS := "未设", "未设"
		if r.License.Minimum != nil {
			minS = strconv.Itoa(*r.License.Minimum)
		}
		if r.License.Maximum != nil {
			maxS = strconv.Itoa(*r.License.Maximum)
		}
		fmt.Fprintf(&b, "- 产品#%d《%s %s》[%s] | 分类：%s | 模块：%s | 部署：%s | 席位：%s–%s | 直购规则：%s\n",
			r.ID, r.Name, r.Version, pub, fallbackDash(r.Category), fallbackDash(strings.Join(r.Modules, "、")),
			fallbackDash(strings.Join(r.DeploymentModes, "/")), minS, maxS, pricing)
	}
	return b.String(), nil
}

func (s server) toolVendorOrders(ctx context.Context, actor data.User, statusFilter string) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	orders := make([]retailOrderRow, 0)
	q := s.db.WithContext(ctx).Table("pl_retail_orders AS orders").
		Select("orders.*, maker.name AS manufacturer, partner.name AS reseller, client.name AS customer").
		Joins("JOIN pl_organizations AS maker ON maker.id = orders.manufacturer_id").
		Joins("JOIN pl_organizations AS partner ON partner.id = orders.reseller_id").
		Joins("JOIN pl_organizations AS client ON client.id = orders.customer_id").
		Where("orders.manufacturer_id = ?", actor.OrganizationID)
	if statusFilter != "" && statusFilter != "all" {
		q = q.Where("orders.status = ?", statusFilter)
	}
	_ = q.Order("orders.id DESC").Limit(40).Scan(&orders).Error
	if len(orders) == 0 {
		return "当前无符合条件的直购订单。", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "直购/代客订单明细（共 %d 笔）：\n", len(orders))
	for _, o := range orders {
		total := float64(o.UnitCents*uint64(o.Quantity)) / 100.0
		comm := float64((o.UnitCents*uint64(o.Quantity)*uint64(o.CommissionBPS)+5000)/10000) / 100.0
		fmt.Fprintf(&b, "- 订单#%d | 产品：%s × %d | 客户：%s | 归属代理商：%s | 订单金额：%.2f元 | 应付佣金：%.2f元 | 状态：%s | 代理催办次数：%d\n",
			o.ID, o.ProductName, o.Quantity, o.Customer, o.Reseller, total, comm, o.Status, o.ReminderCount)
	}
	return b.String(), nil
}

func (s server) toolVendorFinanceAndTickets(ctx context.Context, actor data.User) (string, error) {
	if s.db == nil {
		return "数据库未连接", nil
	}
	maker := actor.OrganizationID
	var b strings.Builder

	finances := make([]struct {
		channelFinanceRequest
		ResellerName string `gorm:"column:reseller_name"`
	}, 0)
	_ = s.db.WithContext(ctx).Table("pl_channel_finance_requests AS f").
		Select("f.*, org.name AS reseller_name").
		Joins("JOIN pl_organizations AS org ON org.id = f.reseller_id").
		Where("f.manufacturer_id = ?", maker).Order("f.id DESC").Limit(25).Scan(&finances).Error

	b.WriteString("【代理商财务申请（预存 / 开票 / 服务费报销）】\n")
	if len(finances) == 0 {
		b.WriteString("- 暂无财务申请记录。\n")
	}
	for _, f := range finances {
		orderRef := ""
		if f.OrderID != nil {
			orderRef = fmt.Sprintf("关联订单#%d ", *f.OrderID)
		}
		fmt.Fprintf(&b, "- 申请#%d | 代理商：%s | 类型：%s | 状态：%s | 金额：%.2f元 | %s抬头/凭证：%s\n",
			f.ID, f.ResellerName, f.Kind, f.Status, float64(f.AmountCents)/100.0, orderRef, fallbackDash(f.Title))
	}

	tickets := make([]struct {
		channelServiceTicket
		ResellerName string `gorm:"column:reseller_name"`
	}, 0)
	_ = s.db.WithContext(ctx).Table("pl_channel_service_tickets AS t").
		Select("t.*, org.name AS reseller_name").
		Joins("JOIN pl_organizations AS org ON org.id = t.reseller_id").
		Where("t.manufacturer_id = ?", maker).Order("t.id DESC").Limit(25).Scan(&tickets).Error

	b.WriteString("\n【渠道售后服务工单】\n")
	if len(tickets) == 0 {
		b.WriteString("- 暂无渠道售后工单。\n")
	}
	for _, t := range tickets {
		fmt.Fprintf(&b, "- 工单#%d《%s》[状态：%s, 版本号：%d] | 代理商：%s | 分类：%s | 问题详情：%s\n",
			t.ID, t.Subject, t.Status, t.Revision, t.ResellerName, t.Category, t.Detail)
	}
	return b.String(), nil
}

func (s server) toolCustomerProjects(ctx context.Context, actor data.User, projectID string) (string, error) {
	if projectID != "" {
		clean := strings.TrimPrefix(projectID, "#")
		idNum, err := strconv.ParseUint(clean, 10, 64)
		if err != nil || idNum == 0 {
			return "无效项目编号：" + projectID, nil
		}
		view, err := s.channel.Get(ctx, actor, idNum)
		if err != nil {
			return "未找到该采购项目或无权访问。", nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "采购项目 #%s《%s》详情：\n- 当前阶段：%s | 待您拍板处理：%v | 下一步：%s | 可执行操作：%s | 期望日期：%s\n- 您的需求原文：%s\n",
			view.ID, view.Title, view.StageLabel, view.IsTodo, view.NextStep, strings.Join(view.Actions, ","),
			fallbackDash(view.DueDate), fallbackDash(ptrStr(view.Requirement)))
		// Strictly only expose SalesQuotes (never ChannelQuotes!)
		if view.Sales != nil && len(*view.Sales) > 0 {
			b.WriteString("- 代理商对客销售报价方案：\n")
			for _, sq := range *view.Sales {
				fmt.Fprintf(&b, "  * 报价单 %s | 总计：%v 元 | 状态：%s | 交付周期：%d 天 | 有效期至：%s | 说明：%s\n",
					sq.ID, sq.Total, sq.Status, sq.DeliveryDays, sq.ValidUntil, fallbackDash(sq.Note))
				for _, line := range sq.Lines {
					fmt.Fprintf(&b, "    - %s × %d（单价 %v 元）\n", line.Label, line.Quantity, line.UnitPrice)
				}
			}
		}
		if view.Order != nil {
			fmt.Fprintf(&b, "- 正式订单编号：%s | 实施交付说明：%s\n", view.Order.ID, fallbackDash(view.ImplementationNote))
		}
		return b.String(), nil
	}

	page, err := s.channel.List(ctx, actor, 0, 50)
	if err != nil {
		return "", err
	}
	if len(page.Items) == 0 {
		return "贵司当前暂无进行中的采购项目。", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "贵司共有 %d 个软件采购项目：\n", len(page.Items))
	for _, p := range page.Items {
		todo := ""
		if p.IsTodo {
			todo = "【待您处理】"
		}
		quoteInfo := "暂无报价"
		if p.Sales != nil && len(*p.Sales) > 0 {
			latest := (*p.Sales)[len(*p.Sales)-1]
			quoteInfo = fmt.Sprintf("最新对客报价 %v 元（%s，有效至 %s）", latest.Total, latest.Status, latest.ValidUntil)
		}
		fmt.Fprintf(&b, "- 项目#%s《%s》| 阶段：%s %s | 可执行操作：%s | %s | 下一步：%s\n",
			p.ID, p.Title, p.StageLabel, todo, strings.Join(p.Actions, ","), quoteInfo, p.NextStep)
	}
	return b.String(), nil
}

func (s server) toolCustomerOrdersAndDealers(ctx context.Context, actor data.User) (string, error) {
	var b strings.Builder
	if links, err := s.channel.Links(ctx, actor); err == nil {
		b.WriteString("【已绑定合作代理商】\n")
		if len(links) == 0 {
			b.WriteString("- 尚未绑定代理商（可在「账号与组织」页面输入代理绑定码建立合作）。\n")
		}
		for _, org := range links {
			fmt.Fprintf(&b, "- 代理商#%d：%s\n", org.ID, org.Name)
		}
	}

	if s.db != nil {
		orders := make([]retailOrderRow, 0)
		_ = s.db.WithContext(ctx).Table("pl_retail_orders AS orders").
			Select("orders.*, maker.name AS manufacturer, partner.name AS reseller, client.name AS customer").
			Joins("JOIN pl_organizations AS maker ON maker.id = orders.manufacturer_id").
			Joins("JOIN pl_organizations AS partner ON partner.id = orders.reseller_id").
			Joins("JOIN pl_organizations AS client ON client.id = orders.customer_id").
			Where("orders.customer_id = ?", actor.OrganizationID).
			Order("orders.id DESC").Limit(30).Scan(&orders).Error

		b.WriteString("\n【厂商软件直购 / 代客订单】\n")
		if len(orders) == 0 {
			b.WriteString("- 暂无软件直购订单。\n")
		}
		for _, o := range orders {
			total := float64(o.UnitCents*uint64(o.Quantity)) / 100.0
			annualTag := "标准授权"
			if o.Annual {
				annualTag = "年度订阅授权"
			}
			fmt.Fprintf(&b, "- 订单#%d | 软件：%s %s × %d份（%s）| 软件厂商：%s | 服务代理商：%s | 应付总额：%.2f元 | 状态：%s\n",
				o.ID, o.ProductName, o.ProductVersion, o.Quantity, annualTag, o.Manufacturer, o.Reseller, total, o.Status)
		}
	}
	return b.String(), nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
