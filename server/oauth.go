package server

type AnthropicTokenResponse struct {
	TokenType    string                `json:"token_type"`
	AccessToken  string                `json:"access_token"`
	ExpiresIn    int                   `json:"expires_in"`
	RefreshToken string                `json:"refresh_token"`
	Scope        string                `json:"scope"`
	Organization AnthropicOrganization `json:"organization"`
	Account      AnthropicAccount      `json:"account"`
}

type AnthropicOrganization struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type AnthropicAccount struct {
	UUID         string `json:"uuid"`
	EmailAddress string `json:"email_address"`
}
