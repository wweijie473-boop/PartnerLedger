package catalog

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

var ErrInvalidKnowledge = errors.New("invalid_product_knowledge")
var ErrInvalidRequirements = errors.New("invalid_requirement")

const maxLicenseCount = 1000000

type Source struct {
	Title     string `json:"title"`
	Reference string `json:"reference"`
	Excerpt   string `json:"excerpt"`
}

type License struct {
	Minimum *int   `json:"minimum"`
	Maximum *int   `json:"maximum"`
	Note    string `json:"note"`
}

type Knowledge struct {
	Category        string   `json:"category"`
	Modules         []string `json:"modules" gorm:"serializer:json"`
	DeploymentModes []string `json:"deployment_modes" gorm:"serializer:json"`
	License         License  `json:"license" gorm:"embedded;embeddedPrefix:license_"`
	Sources         []Source `json:"sources" gorm:"serializer:json"`
}

type ProductKnowledgeProfile struct {
	ProductID uint64 `gorm:"primaryKey;autoIncrement:false"`
	Knowledge `gorm:"embedded"`
	UpdatedBy uint64
	UpdatedAt time.Time
}

type Requirements struct {
	EnterpriseType  string   `json:"enterprise_type"`
	EmployeeCount   *int     `json:"employee_count"`
	RequiredModules []string `json:"required_modules"`
	DeploymentMode  string   `json:"deployment_mode"`
	LicenseCount    *int     `json:"license_count"`
	Version         string   `json:"version"`
}

var deploymentNames = map[string]string{
	"private": "私有化部署", "on_premises": "本地部署", "cloud": "云部署", "saas": "SaaS",
}

func deploymentCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "私有化部署", "私有部署", "私有化":
		return "private"
	case "本地部署", "本地":
		return "on_premises"
	case "云部署", "公有云", "公有云部署":
		return "cloud"
	}
	if _, ok := deploymentNames[value]; ok {
		return value
	}
	return ""
}

func moduleKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "财务管理":
		return "财务"
	case "库存管理":
		return "库存"
	case "采购管理":
		return "采购"
	case "销售管理":
		return "销售"
	}
	return value
}

func EmptyKnowledge() Knowledge {
	return Knowledge{Modules: []string{}, DeploymentModes: []string{}, Sources: []Source{}}
}

func (k *Knowledge) Normalize() error {
	k.Category = strings.TrimSpace(k.Category)
	k.License.Note = strings.TrimSpace(k.License.Note)
	if tooLong(k.Category, 80) || tooLong(k.License.Note, 500) ||
		!validCount(k.License.Minimum) || !validCount(k.License.Maximum) ||
		k.License.Minimum != nil && k.License.Maximum != nil && *k.License.Minimum > *k.License.Maximum {
		return ErrInvalidKnowledge
	}
	var ok bool
	k.Modules, ok = normalizeModules(k.Modules)
	if !ok || len(k.DeploymentModes) > len(deploymentNames) || len(k.Sources) > 10 {
		return ErrInvalidKnowledge
	}
	modes := make([]string, 0, len(k.DeploymentModes))
	seen := make(map[string]bool)
	for _, mode := range k.DeploymentModes {
		code := deploymentCode(mode)
		if code == "" || seen[code] {
			return ErrInvalidKnowledge
		}
		modes, seen[code] = append(modes, code), true
	}
	k.DeploymentModes = modes
	if k.Sources == nil {
		k.Sources = []Source{}
	}
	for i := range k.Sources {
		s := &k.Sources[i]
		s.Title, s.Reference, s.Excerpt = strings.TrimSpace(s.Title), strings.TrimSpace(s.Reference), strings.TrimSpace(s.Excerpt)
		if s.Title == "" || s.Reference == "" || s.Excerpt == "" ||
			tooLong(s.Title, 120) || tooLong(s.Reference, 300) || tooLong(s.Excerpt, 500) {
			return ErrInvalidKnowledge
		}
	}
	return nil
}

func (r *Requirements) Normalize() error {
	r.EnterpriseType, r.DeploymentMode, r.Version = strings.TrimSpace(r.EnterpriseType), strings.TrimSpace(r.DeploymentMode), strings.TrimSpace(r.Version)
	var ok bool
	r.RequiredModules, ok = normalizeModules(r.RequiredModules)
	if !ok || tooLong(r.EnterpriseType, 120) || tooLong(r.DeploymentMode, 80) || tooLong(r.Version, 100) ||
		!validCount(r.EmployeeCount) || !validCount(r.LicenseCount) ||
		r.EnterpriseType == "" && r.EmployeeCount == nil && r.LicenseCount == nil &&
			r.DeploymentMode == "" && r.Version == "" && len(r.RequiredModules) == 0 {
		return ErrInvalidRequirements
	}
	return nil
}

func tooLong(value string, max int) bool {
	return utf8.RuneCountInString(value) > max
}

func validCount(value *int) bool {
	return value == nil || *value >= 1 && *value <= maxLicenseCount
}

func normalizeModules(values []string) ([]string, bool) {
	if len(values) > 30 {
		return nil, false
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, v := range values {
		v = strings.TrimSpace(v)
		key := moduleKey(v)
		if key == "" || tooLong(v, 70) {
			return nil, false
		}
		if !seen[key] {
			result, seen[key] = append(result, v), true
		}
	}
	return result, true
}

func Visible(tx *gorm.DB, actor data.User) *gorm.DB {
	query := tx.Model(&data.Product{})
	switch actor.Organization.Kind {
	case data.Manufacturer:
		return query.Where("manufacturer_id = ?", actor.OrganizationID)
	case data.Reseller:
		return query.Where("published = ?", true)
	default:
		return query.Where("1 = 0")
	}
}
