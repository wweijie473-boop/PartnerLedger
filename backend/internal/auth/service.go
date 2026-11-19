package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const SessionTTL = 12 * time.Hour

var (
	ErrCredentials = errors.New("invalid credentials")
	ErrSession     = errors.New("invalid session")
	usernameFormat = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,63}$`)
)

type Service struct {
	db       *gorm.DB
	sessions *redis.Client
}

func New(db *gorm.DB, sessions *redis.Client) *Service {
	return &Service{db: db, sessions: sessions}
}

// CreateOrganization provisions the first account from the operator CLI.
func CreateOrganization(ctx context.Context, db *gorm.DB, name, kind, username, password string) (uint64, error) {
	name = strings.TrimSpace(name)
	username = normalize(username)
	if name == "" || utf8.RuneCountInString(name) > 150 {
		return 0, errors.New("organization name must contain 1 to 150 characters")
	}
	if kind != data.Manufacturer && kind != data.Reseller && kind != data.Customer {
		return 0, errors.New("organization kind must be MANUFACTURER, RESELLER or CUSTOMER")
	}
	if !usernameFormat.MatchString(username) {
		return 0, errors.New("username must contain 3 to 64 letters, digits, dots, underscores or hyphens")
	}
	if len(password) < 12 || len(password) > 72 {
		return 0, errors.New("password must contain 12 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	org := data.Organization{Name: name, Kind: kind}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		return tx.Create(&data.User{Username: username, OrganizationID: org.ID, PasswordHash: hash}).Error
	})
	if err != nil {
		return 0, err
	}
	return org.ID, nil
}

func LinkCustomer(ctx context.Context, db *gorm.DB, customerID, resellerID uint64) error {
	if customerID == 0 || resellerID == 0 {
		return errors.New("both organization IDs are required")
	}
	var count int64
	if err := db.WithContext(ctx).Model(&data.Organization{}).
		Where("(id = ? AND kind = ?) OR (id = ? AND kind = ?)", customerID, data.Customer, resellerID, data.Reseller).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 2 {
		return errors.New("expected an existing CUSTOMER and RESELLER")
	}
	return db.WithContext(ctx).Exec(`INSERT INTO pl_channel_links (customer_id, reseller_id)
		VALUES (?, ?) ON DUPLICATE KEY UPDATE reseller_id = pl_channel_links.reseller_id`, customerID, resellerID).Error
}

func (s *Service) Login(ctx context.Context, username, password string) (data.User, string, error) {
	var user data.User
	name := normalize(username)
	if !usernameFormat.MatchString(name) || len(password) == 0 || len(password) > 72 {
		return user, "", ErrCredentials
	}
	err := s.db.WithContext(ctx).Preload("Organization").Where("username = ?", name).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, "", ErrCredentials
	}
	if err != nil {
		return user, "", err
	}
	if bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)) != nil {
		return user, "", ErrCredentials
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return user, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	if err := s.sessions.Set(ctx, sessionKey(token), strconv.FormatUint(user.ID, 10), SessionTTL).Err(); err != nil {
		return user, "", err
	}
	return user, token, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (data.User, error) {
	var user data.User
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return user, ErrSession
	}
	id, err := s.sessions.Get(ctx, sessionKey(token)).Uint64()
	if errors.Is(err, redis.Nil) {
		return user, ErrSession
	}
	if err != nil {
		return user, fmt.Errorf("read session: %w", err)
	}
	err = s.db.WithContext(ctx).Preload("Organization").First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, ErrSession
	}
	return user, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Del(ctx, sessionKey(token)).Err()
}

func normalize(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func sessionKey(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "partnerledger:session:" + hex.EncodeToString(hash[:])
}
