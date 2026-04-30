package utils

import (
	"crypto/rand"
	"strings"
)

const OAuthStateAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
const AccessTokenPrefix = "sk-ant-oat01-"
const RefreshTokenPrefix = "sk-ant-ort01-"

func GenerateOAuthState() (string, error) {
	left, err := randomStringFromAlphabet(48, OAuthStateAlphabet)
	if err != nil {
		return "", err
	}

	right, err := randomStringFromAlphabet(43, OAuthStateAlphabet)
	if err != nil {
		return "", err
	}

	var state strings.Builder
	state.Grow(len(left) + 1 + len(right))
	state.WriteString(left)
	state.WriteByte('#')
	state.WriteString(right)
	return state.String(), nil
}

func GenerateRefreshToken() (string, error) {
	suffix, err := randomStringFromAlphabet(95, OAuthStateAlphabet)
	if err != nil {
		return "", err
	}
	return RefreshTokenPrefix + suffix, nil
}

func GenerateAccessToken() (string, error) {
	suffix, err := randomStringFromAlphabet(95, OAuthStateAlphabet)
	if err != nil {
		return "", err
	}
	return AccessTokenPrefix + suffix, nil
}

func randomStringFromAlphabet(length int, alphabet string) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)&63]
	}
	return string(buf), nil
}
