{{ define "config" }}
{
    type: "generate_text",
    description: "This is an example of chain-of-thought (COT) prompting, where the model is guided to think step-by-step to reach a conclusion.", 
    default: true,
    model_provider: "openai",
    model_id: "gpt-4o-mini",
    payload_schema: {
        type: "object",
        properties: {
            numbers: {
                type: "array",
                items: {
                    type: "integer",
                    minimum: 1,
                    maximum: 100
                },
            },
        },
        required: ["numbers"]
    },
}
{{ end }}

You are an excellent calculation agent. Please check if the user's answer is correct.
<role:user/>The sum of the odd numbers in this group will be even: 4, 8, 9, 15, 12, 2, 1.
<role:assistant/>Adding all the odd numbers (9, 15, 1) results in 25. The answer is False.
<role:user/>The sum of the odd numbers in this group will be even: 17, 10, 19, 4, 8, 12, 24.
<role:assistant/>Adding all the odd numbers (17, 19) results in 36. The answer is True.
<role:user/>The sum of the odd numbers in this group will be even: 16, 11, 14, 4, 8, 13, 24.
<role:assistant/>Adding all the odd numbers (11, 13) results in 24. The answer is True.
<role:user/>The sum of the odd numbers in this group will be even: 17, 9, 10, 12, 13, 4, 2.
<role:assistant/>Adding all the odd numbers (17, 9, 13) results in 39. The answer is False.
<role:user/>The sum of the odd numbers in this group will be even: {{ .payload.numbers | join ", " }}.
<role:assistant/>Adding all the odd numbers
