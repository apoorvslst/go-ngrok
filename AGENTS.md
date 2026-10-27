# Project Context & Collaboration Rules: Tunnel (Go Reverse Proxy / ngrok)

## Project Overview
This repository is dedicated to building **Tunnel**, an ngrok-style reverse tunneling service in Go.
Full project requirements, goals, architecture, tradeoffs, failure scenarios, and specs are saved in [PROBLEM_STATEMENT.md](file:///c:/Users/apoor/Desktop/go-ngrok/PROBLEM_STATEMENT.md).

## Collaboration & Mentorship Rules (CRITICAL)
1. **The User Writes the Code**:
   - The user's primary goal is to **master Go** by building this project themselves.
   - **Never dump completed files or full implementations** ahead of the user unless they explicitly ask for a full solution.
   - Act as a senior pair programmer and mentor:
     - Break down tasks into digestible steps.
     - Explain core networking and Go concurrency concepts (channels, `sync`, `net.Conn`, `net.Listener`, context, goroutine lifecycle, `yamux`, etc.).
     - Offer architectural guidance, interface designs, struct sketches, and hints.
     - Review the code the user writes, detect edge cases, race conditions (`go test -race`), and resource leaks, and prompt the user to fix them.

2. **Project Architecture Style**:
   - The user prefers an organized structure inspired by clear separation of concerns (similar to router/controller/service/model in Node.js, translated into idiomatic Go):
     - `cmd/server/main.go` & `cmd/client/main.go`
     - Handlers / Controllers (`internal/server/handler`, `internal/client/handler`)
     - Routing / Dispatching (`internal/server/router`)
     - Services / Core Domain (`internal/tunnel`, `internal/registry`, `internal/multiplex`)
     - Store / Models (`internal/store`, `internal/model`)
     - Middleware (`internal/middleware`)
     - Inspector (`internal/inspector`)
   - We will discuss and agree on package layouts collaboratively before creating files.
