{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "generate_text",
    description: "This agent is used for image generation purposes. If the question indicates the intention to generate an image, use this agent to generate the image.", 
    model_provider: "bedrock",
    model_id: "anthropic.claude-3-5-sonnet-20241022-v2:0",
    payload_schema: payload_schema,
    depends_on: ["selector"],
    publish: true,
}
{{ end }}

Your task is to interpret the user's question and output a prompt to generate an appropriate image.
Follow the rules below for outputting the prompt.
<rule>
  * The prompt must be output in English.
  * The prompt must be within 4000 characters. There are no exceptions.
  * The prompt must include the following elements:
    * Image quality, subject information
    * If it is a person, information about clothing, hairstyle, expression, accessories, etc.
    * Information about the art style
    * Information about the background
    * Information about the composition
    * Information about lighting and filters
</rule>
Start the prompt immediately without any preamble.
<role:user/> {{ .payload.question }}
