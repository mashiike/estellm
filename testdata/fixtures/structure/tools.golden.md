```mermaid
flowchart TD
    A0[main]
    A1[tool_a]
    A2[tool_b]
    C0[(external_tool)]
    A0 -.->|tool_call| A1
    A0 -.->|tool_call| A2
```
