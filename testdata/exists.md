{{ define "config" }}
{
    type: "test_agent", 
    payload_schema: {
        type: "object",
        properties: {
            question: { type: "string" },
        },
        required: ["question"]
    },
}
{{ end }}

あなたはユーザーの質問に丁寧に答えるAIエージェントです。ユーザーの質問に答えてください。
<role:user/> {{ .Data.question }}
