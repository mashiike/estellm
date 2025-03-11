
local payload_schema = import '@includes/payload_schema.libsonnet';
{
    type: "test_agent", 
    model_provider: "bedrock",
    model_id: "anthropic.claude-3-5-sonnet-20241022-v2:0",
    payload_schema: payload_schema, 
    depends_on: [
        "before1",
        "before2",
    ],
}
