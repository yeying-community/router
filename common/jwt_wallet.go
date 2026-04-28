package common

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yeying-community/router/common/config"
)

// WalletClaims defines JWT claims for wallet login.
type WalletClaims struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	TokenType     string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

// GenerateWalletJWT issues a JWT for the given user id and wallet address.
func GenerateWalletJWT(userID string, walletAddress string) (token string, expiresAt time.Time, err error) {
	secret := []byte(config.JWTSecret)
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("auth.jwt_secret not configured")
	}
	expiresAt = time.Now().Add(time.Duration(config.JWTExpireHours) * time.Hour)
	claims := WalletClaims{
		UserID:        userID,
		WalletAddress: walletAddress,
		TokenType:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   walletAddress,
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = tokenObj.SignedString(secret)
	return
}

// VerifyWalletJWT validates token and returns claims.
func VerifyWalletJWT(tokenString string) (*WalletClaims, error) {
	claims, err := verifyWithSecrets(tokenString, append([]string{config.JWTSecret}, config.JWTFallbackSecrets...))
	if err != nil {
		return nil, err
	}
	if claims.TokenType == "refresh" {
		return nil, errors.New("refresh token not allowed for access")
	}
	return claims, nil
}

// GenerateWalletRefreshJWT issues a refresh token for the given user id and wallet address.
func GenerateWalletRefreshJWT(userID string, walletAddress string) (token string, expiresAt time.Time, err error) {
	secret := []byte(config.JWTSecret)
	if len(secret) == 0 {
		return "", time.Time{}, errors.New("auth.jwt_secret not configured")
	}
	expiresAt = time.Now().Add(time.Duration(config.RefreshTokenExpireHours) * time.Hour)
	claims := WalletClaims{
		UserID:        userID,
		WalletAddress: walletAddress,
		TokenType:     "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   walletAddress,
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = tokenObj.SignedString(secret)
	return
}

// VerifyWalletRefreshJWT validates refresh token and returns claims.
func VerifyWalletRefreshJWT(tokenString string) (*WalletClaims, error) {
	claims, err := verifyWithSecrets(tokenString, append([]string{config.JWTSecret}, config.JWTFallbackSecrets...))
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "refresh" {
		return nil, errors.New("token is not refresh")
	}
	return claims, nil
}

// verifyWithSecrets tries multiple secrets in order and returns on first success.
func verifyWithSecrets(tokenString string, secrets []string) (*WalletClaims, error) {
	if len(secrets) == 0 {
		return nil, errors.New("auth.jwt_secret not configured")
	}
	var lastErr error
	for _, sec := range secrets {
		secBytes := []byte(sec)
		if len(secBytes) == 0 {
			continue
		}
		parsed, err := jwt.ParseWithClaims(tokenString, &WalletClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secBytes, nil
		})
		if err != nil {
			lastErr = err
			continue
		}
		if claims, ok := parsed.Claims.(*WalletClaims); ok && parsed.Valid {
			return claims, nil
		}
		lastErr = errors.New("invalid token")
	}
	if lastErr == nil {
		lastErr = errors.New("invalid token")
	}
	return nil, lastErr
}
