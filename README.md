# cloudcut-media-server

Cloud-backed media processing server for video editing with GCP integration

> Built with [Agent Context Protocol](https://github.com/prmichaelsen/agent-context-protocol)

## Quick Start

[Add installation and usage instructions here]

## Features

- GCP-backed video transcoding and rendering
- WebSocket persistent connections for real-time status
- Asset management via Cloud Storage
- AI-powered video analysis via Vertex AI

## Development

This project uses the Agent Context Protocol for development:

- `@acp.init` - Initialize agent context
- `@acp.plan` - Plan milestones and tasks
- `@acp.proceed` - Continue with next task
- `@acp.status` - Check project status

See [AGENT.md](./AGENT.md) for complete ACP documentation.

## Project Structure

```
cloudcut-media-server/
├── AGENT.md              # ACP methodology
├── agent/                # ACP directory
│   ├── design/          # Design documents
│   ├── milestones/      # Project milestones
│   ├── tasks/           # Task breakdown
│   ├── patterns/        # Architectural patterns
│   └── progress.yaml    # Progress tracking
└── (your project files)
```

## Getting Started

1. Initialize context: `@acp.init`
2. Plan your project: `@acp.plan`
3. Start building: `@acp.proceed`

## License

MIT

## Author

Patrick Michaelsen
