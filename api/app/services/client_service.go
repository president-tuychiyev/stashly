package services

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// passwordAlphabet leaves out characters that are easy to misread.
const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ClientInput carries the writable fields of a client. Nil means "leave as is".
type ClientInput struct {
	Name         *string
	Username     *string
	Password     *string
	Status       *string
	QuotaBytes   *int64
	AllowedMimes *[]string
	MaxFileSize  *int64
	// OwnerID is the admin that manages the client.
	OwnerID *uint
	// ClearQuotaBytes and ClearMaxFileSize reset the matching column to null.
	ClearQuotaBytes  bool
	ClearMaxFileSize bool
}

type ClientService struct{}

func NewClientService() *ClientService {
	return &ClientService{}
}

// GeneratePassword returns a random 16 character password.
func GeneratePassword() (string, error) {
	out := make([]byte, 16)
	max := big.NewInt(int64(len(passwordAlphabet)))

	for index := range out {
		pick, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[index] = passwordAlphabet[pick.Int64()]
	}

	return string(out), nil
}

// Create stores a new client and returns it together with the plain password,
// which is the only moment that value is ever available.
func (r *ClientService) Create(input ClientInput, creatorID *uint) (*models.Client, string, error) {
	if input.Username == nil || *input.Username == "" {
		return nil, "", errors.New("username is required")
	}

	plain := ""
	if input.Password != nil && *input.Password != "" {
		plain = *input.Password
	} else {
		generated, err := GeneratePassword()
		if err != nil {
			return nil, "", err
		}
		plain = generated
	}

	hashed, err := facades.Hash().Make(plain)
	if err != nil {
		return nil, "", err
	}

	client := &models.Client{
		Username:     input.Username,
		Name:         input.Name,
		Password:     hashed,
		Status:       "active",
		AllowedMimes: []string{},
		OwnerID:      input.OwnerID,
		CreatorID:    creatorID,
		UpdaterID:    creatorID,
	}

	if input.Status != nil && *input.Status != "" {
		client.Status = *input.Status
	}
	if input.QuotaBytes != nil {
		client.QuotaBytes = input.QuotaBytes
	}
	if input.AllowedMimes != nil {
		client.AllowedMimes = *input.AllowedMimes
	}
	if input.MaxFileSize != nil {
		client.MaxFileSize = input.MaxFileSize
	}

	if err := facades.Orm().Query().Create(client); err != nil {
		return nil, "", err
	}

	r.ForgetCache(client)

	return client, plain, nil
}

// Update applies the supplied fields to an existing client.
func (r *ClientService) Update(client *models.Client, input ClientInput, updaterID *uint) error {
	previousUsername := client.Username

	if input.Name != nil {
		client.Name = input.Name
	}
	if input.Username != nil && *input.Username != "" {
		client.Username = input.Username
	}
	if input.Status != nil && *input.Status != "" {
		client.Status = *input.Status
	}
	if input.QuotaBytes != nil {
		client.QuotaBytes = input.QuotaBytes
	}
	if input.AllowedMimes != nil {
		client.AllowedMimes = *input.AllowedMimes
	}
	if input.MaxFileSize != nil {
		client.MaxFileSize = input.MaxFileSize
	}
	if input.OwnerID != nil && *input.OwnerID != 0 {
		client.OwnerID = input.OwnerID
	}
	if input.ClearQuotaBytes {
		client.QuotaBytes = nil
	}
	if input.ClearMaxFileSize {
		client.MaxFileSize = nil
	}
	client.UpdaterID = updaterID

	if err := facades.Orm().Query().Save(client); err != nil {
		return err
	}

	r.forgetUsername(previousUsername)
	r.ForgetCache(client)

	return nil
}

// ResetPassword sets a new password, generating one when none is supplied.
func (r *ClientService) ResetPassword(client *models.Client, password *string, updaterID *uint) (string, error) {
	plain := ""
	if password != nil && *password != "" {
		plain = *password
	} else {
		generated, err := GeneratePassword()
		if err != nil {
			return "", err
		}
		plain = generated
	}

	hashed, err := facades.Hash().Make(plain)
	if err != nil {
		return "", err
	}

	client.Password = hashed
	client.UpdaterID = updaterID

	if err := facades.Orm().Query().Save(client); err != nil {
		return "", err
	}

	r.ForgetCache(client)

	return plain, nil
}

// ForgetCache drops everything the api_check middleware caches for a client:
// the row itself and the digest that lets it skip bcrypt. Both have to go
// together, otherwise a reset password would still be accepted, or a blocked
// client would keep passing, until the TTL ran out.
func (r *ClientService) ForgetCache(client *models.Client) {
	if client != nil {
		r.forgetUsername(client.Username)
	}
}

func (r *ClientService) forgetUsername(username *string) {
	if username == nil || *username == "" {
		return
	}

	facades.Cache().Forget(ClientCacheKey(*username))
	facades.Cache().Forget(ClientAuthCacheKey(*username))
}
