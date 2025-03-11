{{ define "config" }}
{
    type: "generate_image", 
    model_provider: "openai",
    model_id: "dall-e-3",
    description: "This agent generates images using DALL-E 3."
}
{{ end }}

{{ (ref `genimage_prompt`).result }}
