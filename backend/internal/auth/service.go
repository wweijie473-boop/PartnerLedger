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
func CreateOrganization(ctx context.Context, db *gorm.DB, name, kind, username, password string) error {
	name = strings.TrimSpace(name)
	username = normalize(username)
	if name == "" || utf8.RuneCountInString(name) > 150 {
		return errors.New("organization name must contain 1 to 150 characters")
	}
	if kind != data.Manufacturer && kind != data.Reseller {
		return errors.New("organization kind must be MANUFACTURER or RESELLER")
	}
	if !usernameFormat.MatchString(username) {
		return errors.New("username must contain 3 to 64 letters, digits, dots, underscores or hyphens")
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("password must contain 12 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org := data.Organization{Name: name, Kind: kind}
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		return tx.Create(&data.User{Username: username, OrganizationID: org.ID, PasswordHash: hash}).Error
	})
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
