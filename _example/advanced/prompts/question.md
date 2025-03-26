{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "generate_text", 
    description: "This agent is used for general purposes.",
    model_provider: "openai",
    model_id: "gpt-4o-mini",
    payload_schema: payload_schema,
    tools: [
        "weather"
    ],
    publish: true,
}
{{ end }}

You are an AI agent that answers user questions politely. Please answer the user's question.
<role:user/> {{ .payload.question }}
