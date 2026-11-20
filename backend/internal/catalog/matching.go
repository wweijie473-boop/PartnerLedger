package catalog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

type Check struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Match struct {
	Product      data.Product `json:"product"`
	Category     string       `json:"category"`
	Status       string       `json:"status"`
	Checks       []Check      `json:"checks"`
	Sources      []Source     `json:"sources"`
	met, missing int
}

type candidateRow struct {
	data.Product `gorm:"embedded"`
	Knowledge    `gorm:"embedded"`
}

func MatchPublished(tx *gorm.DB, actor data.User, requirement Requirements) ([]Match, error) {
	var rows []candidateRow
	const columns = "pl_products.*, k.category, k.modules, k.deployment_modes, k.license_minimum, k.license_maximum, k.license_note, k.sources"
	if err := Visible(tx, actor).Where("pl_products.published = ?", true).
		Select(columns).Joins("JOIN pl_product_knowledge_profiles k ON k.product_id = pl_products.id").
		Order("pl_products.id").Find(&rows).Error; err != nil {
		return nil, err
	}
	results := make([]Match, 0, len(rows))
	for _, row := range rows {
		results = append(results, evaluate(row.Product, row.Knowledge, requirement))
	}
	slices.SortStableFunc(results, func(a, b Match) int {
		if (a.missing == 0) != (b.missing == 0) {
			if a.missing == 0 {
				return -1
			}
			return 1
		}
		if a.met != b.met {
			return b.met - a.met
		}
		return a.missing - b.missing
	})
	return results[:min(8, len(results))], nil
}

func evaluate(product data.Product, knowledge Knowledge, requirement Requirements) Match {
	match := Match{Product: product, Category: knowledge.Category, Sources: knowledge.Sources}
	add := func(field, value, status, detail string) {
		match.Checks = append(match.Checks, Check{field, value, status, detail})
		switch status {
		case "met":
			match.met++
		case "missing":
			match.missing++
		}
	}
	for _, requested := range requirement.RequiredModules {
		switch {
		case len(knowledge.Modules) == 0:
			add("module", requested, "unknown", "目录未录入模块清单")
		case slices.ContainsFunc(knowledge.Modules, func(module string) bool { return moduleKey(module) == moduleKey(requested) }):
			add("module", requested, "met", "目录模块清单包含此项")
		default:
			add("module", requested, "missing", "目录模块清单中未找到此项")
		}
	}
	if requirement.DeploymentMode != "" {
		code := deploymentCode(requirement.DeploymentMode)
		switch {
		case code == "" || len(knowledge.DeploymentModes) == 0:
			add("deployment", requirement.DeploymentMode, "unknown", "需核对需求措辞或产品部署资料")
		case slices.Contains(knowledge.DeploymentModes, code):
			add("deployment", requirement.DeploymentMode, "met", "目录标记支持此部署方式")
		default:
			add("deployment", requirement.DeploymentMode, "missing", "目录部署清单中未找到此方式")
		}
	}
	if requirement.Version != "" {
		if strings.EqualFold(product.Version, requirement.Version) {
			add("version", requirement.Version, "met", "产品版本与要求一致")
		} else {
			add("version", requirement.Version, "missing", "产品版本与要求不同")
		}
	}
	checkLicense(add, knowledge.License, requirement.LicenseCount)
	if len(knowledge.Sources) == 0 {
		add("source", "", "unknown", "厂家尚未补充可引用的资料")
	}
	switch {
	case match.missing > 0:
		match.Status = "mismatch"
	case match.met == len(match.Checks):
		match.Status = "matched"
	default:
		match.Status = "needs_confirmation"
	}
	return match
}

func checkLicense(add func(string, string, string, string), rule License, count *int) {
	if count == nil {
		add("license", "", "unknown", "请确认授权席位数量")
		return
	}
	value := fmt.Sprint(*count)
	switch {
	case rule.Minimum != nil && *count < *rule.Minimum:
		add("license", value, "missing", fmt.Sprintf("少于目录记录的最低授权数量 %d", *rule.Minimum))
	case rule.Maximum != nil && *count > *rule.Maximum:
		add("license", value, "missing", fmt.Sprintf("超过目录记录的最高授权数量 %d", *rule.Maximum))
	case rule.Minimum == nil || rule.Maximum == nil:
		add("license", value, "unknown", "目录授权范围未录全")
	default:
		add("license", value, "met", "授权数量在目录记录范围内")
	}
}
