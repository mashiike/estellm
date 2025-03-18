{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "generate_text", 
    description: "This agent is used for general purposes.",
    model_provider: "bedrock",
    model_id: "anthropic.claude-3-5-sonnet-20241022-v2:0",
    payload_schema: payload_schema,
    depends_on: ["selector"],
    tools: [
        "weather"
    ],
    publish: true,
}
{{ end }}

You are an AI agent that answers user questions politely. Please answer the user's question.
<role:user/> {{ .payload.question }}
