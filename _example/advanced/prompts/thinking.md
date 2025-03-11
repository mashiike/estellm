{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "generate_text", 
    description: "This agent answers questions thoughtfully. It is used when mathematical knowledge or difficult explanations and summaries are required.",
    model_provider: "bedrock",
    model_id: "us.anthropic.claude-3-7-sonnet-20250219-v1:0",
    model_params: {
        max_tokens: 24000,
        temperature: 1.0,
        thinking: {"type": "enabled", "budget_tokens": 16000},
    },
    payload_schema: payload_schema,
    depends_on: ["selector"],
    tools: [
        "weather"
    ],
}
{{ end }}

You are an AI agent that answers user questions politely. Please answer the user's question.
<role:user/> {{ .payload.question }}
