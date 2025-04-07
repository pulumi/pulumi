package apitype

import "time"

type PublicKey struct {
	Type        string     `json:"type"`
	Fingerprint string     `json:"fingerprint"`
	LastUsed    *time.Time `json:"lastUsed,omitempty"`
}

type CreatePublicKeyRequest struct {
	PublicKey []byte `json:"publicKey"`
}

type ListPublicKeysResponse struct {
	PublicKeys []PublicKey `json:"publicKeys,omitempty"`
	NextToken  string      `json:"nextToken,omitempty"`
}

type CreatePubkeyExchangeSessionRequest struct {
	PublicKey []byte `json:"publicKey"`
}

type CreatePubkeyExchangeSessionResponse struct {
	SessionToken string `json:"sessionToken"`
}

type OAuth2TokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
