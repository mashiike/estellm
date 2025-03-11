{
  type: "object",
  properties: {
    next_agent: {
      type: "string",
      description: |||
        The identifier of the next agent to be called in the workflow.

        This is determined based on the analysis of the input request.
        The agent should be a valid name that corresponds to a predefined agent
        capable of handling the request.

        If the system is unable to determine a suitable agent with sufficient confidence,
        it may return an empty string or a fallback agent (if applicable).
      |||,
    },
    reasoning: {
      type: "string",
      description: |||
        A textual explanation of why the system selected the given `next_agent`.

        This explanation helps to understand the decision-making process of the system,
        and can provide insights into how the input was interpreted.

        The reasoning may include:
        - Key factors that influenced the agent selection
        - How the intent of the request was interpreted
        - Any ambiguities or uncertainties in the decision

        It is recommended that `reasoning` is provided in a human-readable format
        and structured if necessary (e.g., Markdown for better readability).
      |||,
      example: "The user's request mentioned 'refund' and 'payment issue', which aligns best with the `order_support` agent."
    },
    confidence: {
      type: "number",
      minimum: 0.0,
      maximum: 1.0,
      description: |||
        A confidence score (0.0 to 1.0) indicating the certainty of the decision.

        - **[0.0, 0.2):** Very low confidence. Unable to determine a decision.
        - **[0.2, 0.4):** Low confidence. The intent of the request is unclear.
        - **[0.4, 0.6):** Moderate confidence. The intent is understood, but the appropriate agent cannot be determined.
        - **[0.6, 0.8):** High confidence. The intent and appropriate agent are identified, though other agents may be possible.
        - **[0.8, 1.0]:** Very high confidence. The selected agent is highly likely to be correct.
      |||,
      example: 0.75
    }
  },
  required: ["next_agent", "reasoning", "confidence"]
}
