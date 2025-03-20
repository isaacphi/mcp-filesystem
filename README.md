# MCP Filesystem

A Model Context Protocol (MCP) server that exposes resources for each file in a working directory and sends change notifications.

## Status

⚠️ Pre-beta Quality ⚠️

"It works on my machine". Issues are welcome ❤️

## Features

- **Resources**: Creates one MCP resource for each file in your workspace
- **Gitignore Support**: Respects `.gitignore` rules
- **Change Notification**: Detects file changes, additions, and deletions
- **MIME Type Detection and Encoding Handling**: Identifies file types and handles various text encodings

## Setup

Install Go: Follow instructions at <https://golang.org/doc/install>

Fetch or update this server:

```bash
go install github.com/isaacphi/mcp-filesystem@latest
```

Add the following to your client configuration (located at `~/Library/Application Support/Claude/claude_desktop_config.json` for Claude Desktop):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem",
      "args": ["--workspace", "/path/to/your/repository"]
    }
  }
}
```

Replace `/path/to/your/repository` with the absolute path to your project directory.

## Usage

Your client will be able to access and reference all non-ignored files in your repository as MCP resources. Each file is registered as a separate resource with appropriate MIME type detection.

### Client Requirements

Your client needs to support the following MCP features:

Resource Listing: The ability to list and access resources exposed by the server
Change Notifications: Support for receiving `notifications/resources/list_changed` events
Resource Content Access: Ability to request and render resource content with appropriate MIME types

## About

This project uses:

- [mcp-golang](https://github.com/metoro-io/mcp-golang) for MCP communication
- [fsnotify](https://github.com/fsnotify/fsnotify) for file system event monitoring
- [go-gitignore](https://github.com/sabhiram/go-gitignore) for parsing `.gitignore` files

## Development

Clone the repository:

```bash
git clone https://github.com/isaacphi/mcp-filesystem.git
cd mcp-filesystem
```

Install dependencies:

```bash
go mod download
```

Build:

```bash
go build
```

Configure you client to use your local build:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/full/path/to/your/mcp-filesystem/mcp-filesystem",
      "args": ["--workspace", "/path/to/repository"],
      "env": {
        "DEBUG": "1"
      }
    }
  }
}
```

## Feedback

Please submit issues with as many details as you can.

Set the `DEBUG` environment variable to enable verbose logging:

```json
"env": {
  "DEBUG": "1"
}
```

## Features on my radar

- [x] Resources for each file in workspace
- [x] .gitignore support
- [x] Change notifications
- [ ] Optional support for line numbers
- [ ] Additional ignore patterns (beyond `.gitignore`)
- [ ] Debounced notifications for high-volume file changes
- [ ] info, create, edit, and delete tools
