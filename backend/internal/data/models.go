package data

const (
	Manufacturer = "MANUFACTURER"
	Reseller     = "RESELLER"
)

type Organization struct {
	ID   uint64 `json:"id,string"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type User struct {
	ID             uint64       `json:"id,string"`
	OrganizationID uint64       `json:"-"`
	Organization   Organization `json:"organization"`
	Username       string       `json:"username"`
	PasswordHash   []byte       `json:"-"`
}

type Product struct {
	ID             uint64 `json:"id,string"`
	ManufacturerID uint64 `json:"manufacturer_id,string"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	Description    string `json:"description"`
	Published      bool   `json:"published"`
}
