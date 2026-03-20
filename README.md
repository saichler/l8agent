# l8agent

Generic & Secure AI Agent library for Layer 8 ecosystem projects.

l8agent provides AI-powered chat orchestration with built-in data masking, tool execution, and conversation persistence. Consumer projects (l8erp, l8bugs, etc.) integrate this library to add AI assistant capabilities to their systems.

## Architecture

```
Client POST /agent/{area}/AgntChat
└─ Chat Orchestrator
   ├─ Load/Create Conversation
   ├─ Mask User Input (PII, financial data)
   ├─ Tool Loop (max 10 iterations)
   │  ├─ Send to Claude (Anthropic API)
   │  ├─ Execute Tool Calls via vnic
   │  │  └─ l8query, create, update, delete, list_modules, describe_model
   │  └─ Mask Tool Results
   ├─ Unmask Final Response
   ├─ Persist Messages (PostgreSQL)
   └─ Return Response
```

## Project Structure

```
l8agent/
├── go/
│   ├── init.go                  # Consumer API: Initialize() and InitializeChat()
│   ├── executor/                # Executes LLM tool calls against Layer 8 services
│   ├── llm/                     # Claude API HTTP client
│   ├── masking/                 # Data masking/unmasking proxy
│   │   ├── config.go            # Field classification rules
│   │   ├── proxy.go             # Mask/unmask operations
│   │   └── tokenmap.go          # Per-request token mapping
│   ├── schema/                  # Schema provider for LLM system prompt
│   ├── services/
│   │   ├── chat/                # Chat orchestration (non-persisted facade)
│   │   ├── conversations/       # Conversation metadata CRUD (PostgreSQL)
│   │   ├── messages/            # Chat message persistence (PostgreSQL)
│   │   └── prompts/             # Prompt template CRUD (PostgreSQL)
│   ├── tools/                   # LLM tool definitions (6 tools)
│   └── types/l8agent/           # Generated protobuf types
├── proto/
│   └── l8agent.proto            # Protobuf definitions
└── js/
    ├── l8agent-chat.js          # Desktop chat UI component
    ├── l8agent-chat.css         # Chat styling (layer8d theme tokens)
    ├── l8agent-enums.js         # Enum definitions & renderers
    ├── l8agent-forms.js         # Form configurations
    ├── l8agent-columns.js       # Table column definitions
    └── m/                       # Mobile chat UI
        ├── l8agent-chat-m.js
        └── l8agent-chat-m.css
```

## Services

| Service | ServiceName | Purpose | Storage |
|---------|-------------|---------|---------|
| Conversations | `AgntConvo` | Conversation metadata (title, status, timestamps) | PostgreSQL |
| Messages | `AgntMsg` | Individual chat messages (user + assistant) | PostgreSQL |
| Prompts | `AgntPrmpt` | Reusable system prompt templates | PostgreSQL |
| Chat | `AgntChat` | Orchestration facade (no direct persistence) | In-memory |

## LLM Tools

The agent exposes 6 tools to Claude:

| Tool | Description |
|------|-------------|
| `l8query` | Execute SELECT queries with filtering, aggregates, GROUP BY |
| `create_record` | POST a new record to a service endpoint |
| `update_record` | PUT an updated record to a service endpoint |
| `delete_record` | DELETE a record via query |
| `list_modules` | Return the service catalog (Tier 1 schema) |
| `describe_model` | Return field definitions for a model (Tier 2 schema) |

## Data Masking

All data flowing through the agent is classified and masked before reaching the LLM:

| Classification | Examples | Masking |
|----------------|----------|---------|
| Always Mask | SSN, tax ID, bank account | Replaced with `[MASKED]` |
| Mask Names | Names, emails, phones | Tokenized: `[NAME_1]`, `[NAME_2]` |
| Mask Money | Salaries, amounts, prices | Tokenized: `[MONEY_1]`, `[MONEY_2]` |
| Never Mask | IDs, dates, codes, enums | Passed through unchanged |

Token maps are per-request — masked values are unmasked only in the final response within the same request lifecycle. Consumer projects can provide custom field classification overrides.

## Integration

```go
import l8agent "github.com/saichler/l8agent/go"

config := l8agent.AgentConfig{
    Resources:        res,
    ServiceArea:      99,
    DBCreds:          "db-creds",
    DBName:           "agent_db",
    MaskingOverrides: customMaskFunc,       // optional
    DefaultPrompts:   []*l8agent.L8AgentPrompt{...}, // optional
}

// Phase 1: Activate ORM services (parallel with other services)
l8agent.Initialize(config, vnic)

// Phase 2: Activate chat (after introspector is fully populated)
l8agent.InitializeChat(config, vnic)
```

Initialization is split into two phases because the chat service's schema provider needs the introspector to have all project types registered before it can build the system prompt.

## Environment

Requires `ANTHROPIC_API_KEY` environment variable for Claude API access.

## License

Apache License, Version 2.0 - (c) 2025 Sharon Aicler (saichler@gmail.com)
