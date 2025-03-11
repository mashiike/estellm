{{ define "config" }}
local env = std.native('env');
{
    type: "generate_text",
    description: "This is an example of a remote tool.", 
    default: true,
    model_provider: "openai",
    model_id: "gpt-4o-mini",
    tools: [
        env("REMOTE_TOOL_URL", "http://localhost:8088"),
    ],
}
{{ end }}

I will ask two questions. Please answer each one.
How much is 20 USD in JPY now?
How many euros is 3000 JPY?
