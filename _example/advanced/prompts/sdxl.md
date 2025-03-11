{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "generate_image",
    description: "This agent generates images using SDXL. It is executed when the payload contains a prompt and the question indicates the intention to generate an image using SDXL.",
    model_provider: "bedrock",
    model_id: "stability.stable-diffusion-xl-v1",
    payload_schema: payload_schema + {
        type: "object",
        properties: {
            prompt: { type: "string" },
            negative_prompt: { type: "string" },
        },
        required: ["prompt"],
    },
    depends_on: ["selector"],
}
{{ end }}

{{ .payload | toJson }}
