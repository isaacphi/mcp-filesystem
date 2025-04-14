module github.com/isaacphi/mcp-filesystem

go 1.24.0

require (
	github.com/fsnotify/fsnotify v1.6.0
	github.com/mark3labs/mcp-go v0.20.0
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	golang.org/x/text v0.12.0
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kisielk/errcheck v1.9.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/telemetry v0.0.0-20240522233618-39ace7a40ae7 // indirect
	golang.org/x/tools v0.30.0 // indirect
	golang.org/x/vuln v1.1.4 // indirect
	honnef.co/go/tools v0.6.1 // indirect
)

tool (
	github.com/kisielk/errcheck
	golang.org/x/vuln/cmd/govulncheck
	honnef.co/go/tools/cmd/staticcheck
)
