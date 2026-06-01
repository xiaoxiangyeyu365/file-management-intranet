package service

import "cloudbox/internal/util/crypto"

type cryptoAdapter struct{}

func NewCryptoAdapter() *cryptoAdapter {
	return &cryptoAdapter{}
}

func (c *cryptoAdapter) HashPassword(password string) (string, error) {
	return crypto.HashPassword(password)
}

func (c *cryptoAdapter) CheckPassword(password, hash string) bool {
	return crypto.CheckPassword(password, hash)
}

func (c *cryptoAdapter) GenerateToken(userID int64, username, role string) (string, error) {
	return crypto.GenerateToken(userID, username, role)
}
