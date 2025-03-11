```mermaid
flowchart TD
    A0[main]
    A1[start]
    A2[tool_a]
    A3[tool_b]
    A0 -.->|tool_call| A2
    A0 -.->|tool_call| A3
    A1 --> A0
    A2 --> A1
    A3 --> A1
```
