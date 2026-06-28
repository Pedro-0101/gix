package app

import "gix/internal/ai"

// newProvider builds the AI client the services talk to. It is the single seam
// where a future backend (Anthropic-direct, OpenAI-direct, a local model) gets
// selected — today every configured model is served through OpenRouter, so
// there is one concrete client. Keeping the choice here, instead of calling
// ai.New from each service's wiring in shell.go, means a new provider is added
// in one place. See docs/todo for the registry-by-provider follow-up.
func newProvider(apiKey string) *ai.Client {
	return ai.New(apiKey)
}
