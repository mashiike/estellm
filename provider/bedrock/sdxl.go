package bedrock

import "encoding/json"

// https://docs.aws.amazon.com/ja_jp/bedrock/latest/userguide/model-parameters-diffusion-1-0-text-image.html
type StableDiffusionXLRequset struct {
	TextPrompts []StableDiffusionXLTextPrompt `json:"text_prompts"`
	Height      int                           `json:"height,omitempty"`
	Width       int                           `json:"width,omitempty"`
	StylePreset string                        `json:"style_preset,omitempty"`
	Seed        int64                         `json:"seed,omitempty"`
	CFGScale    float64                       `json:"cfg_scale,omitempty"`
	Steps       int                           `json:"steps,omitempty"`
	Sampler     string                        `json:"sampler,omitempty"`
	Samples     int                           `json:"samples,omitempty"`
	Extra       json.RawMessage               `json:"extra,omitempty"`
}

type StableDiffusionXLTextPrompt struct {
	Text   string  `json:"text"`
	Weight float64 `json:"weight"`
}

type StableDiffusionXLResponse struct {
	Result    string                      `json:"result"`
	Artifacts []StableDiffusionXLArtifact `json:"artifacts"`
}

type StableDiffusionXLArtifact struct {
	Seed         int64  `json:"seed"`
	Base64       string `json:"base64"`
	FinishReason string `json:"finishReason"`
}

type StableDiffusionXLJSONPrompt struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	StylePreset    string `json:"style_preset"`
}
