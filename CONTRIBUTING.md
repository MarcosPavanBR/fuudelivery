# Contributing to FuuDelivery

Thank you for considering contributing to FuuDelivery! This guide will help you get started.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Coding Conventions](#coding-conventions)
- [Testing](#testing)
- [CI Pipeline](#ci-pipeline)
- [Payment System](#payment-system-fuupayment)
- [Pull Request Process](#pull-request-process)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Troubleshooting](#troubleshooting)
- [Code Review Checklist](#code-review-checklist)

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| **Go** | 1.23+ | Backend development |
| **Node.js** | 20+ | Frontend development |
| **Docker** | Latest | MongoDB for local development |
| **Git** | Latest | Version control |

### Backend Setup

```bash
# Clone the repository
git clone https://github.com/MarcosPavanBR/fuudelivery.git
cd fuudelivery

# Start MongoDB via Docker
docker run -d --name mongodb -p 27017:27017 mongo:6

# Set up the monolith
cd cmd/fuudelivery
go mod tidy
go run main.go
```

### Frontend Setup

```bash
# Web Restaurant (React + Tailwind)
cd Frontend/WebRestaurant
npm install
npm start

# Payment Panel (HTML/JS) — ARQUIVADO em legacy/PaymentPanel (2026-08).
# Funcionalidade equivalente vive na aba Financeiro do WebAdmin.

# Mobile Apps (React Native/Expo)
cd Frontend/AppComida
npm install
npx expo start

cd Frontend/AppEntrega
npm install
npx expo start
```

### Environment Variables

Copy the `.env.example` files in each frontend directory and configure them:

```bash
cp Frontend/AppComida/.env.example Frontend/AppComida/.env.local
cp Frontend/AppEntrega/.env.example Frontend/AppEntrega/.env.local
cp Frontend/WebAdmin/.env.example Frontend/WebAdmin/.env.local
cp Frontend/WebRestaurant/.env.example Frontend/WebRestaurant/.env.local
```

See [README.md](README.md#variáveis-de-ambiente) for full variable documentation.

## First-Time Contributors

Welcome! We appreciate your interest in contributing to FuuDelivery. Here is a step-by-step guide to make your first contribution.

### Finding an Issue

Browse issues labeled for newcomers:

- 🔰 **Good First Issues** — [github.com/MarcosPavanBR/fuudelivery/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22](https://github.com/MarcosPavanBR/fuudelivery/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- 🤝 **Help Wanted** — [github.com/MarcosPavanBR/fuudelivery/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22](https://github.com/MarcosPavanBR/fuudelivery/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)

If no suitable issue exists, check the [Troubleshooting](#troubleshooting) section for known pain points, or open a new issue describing what you would like to work on.

### Your First Contribution in 7 Steps

#### 1. Fork the Repository

```bash
# Click the "Fork" button on GitHub, then clone your fork
git clone https://github.com/<YOUR_USERNAME>/fuudelivery.git
cd fuudelivery
```

#### 2. Set Up Your Environment

Follow the [Development Setup](#development-setup) instructions above to install Go, Node.js, Docker, and configure environment variables.

#### 3. Create a Branch

```bash
# Always branch from master
git checkout master
git pull origin master

# Create your feature branch
git checkout -b fix/typo-in-readme
# or
git checkout -b feat/add-dark-mode-toggle
```

#### 4. Make Your Changes

- Follow the [Coding Conventions](#coding-conventions) for Go or TypeScript
- Keep changes small and focused — one issue per PR
- Add or update tests if you are changing behavior
- Update documentation if you are changing public APIs or adding features

#### 5. Run the CI Checks Locally

```bash
# Go changes
cd <module-dir>  # e.g., cmd/fuudelivery
go mod tidy
go build ./...
go vet ./...
go test ./...
gofmt -s -w .

# Frontend changes
cd Frontend/WebRestaurant  # or the relevant frontend
npm install
npm test
```

See the [CI Pipeline](#ci-pipeline) section for the full list of checks.

#### 6. Commit and Push

```bash
git add .
git commit -m "fix(readme): correct typo in installation guide"
git push origin fix/typo-in-readme
```

Use [Conventional Commits](#commit-message-guidelines) format: `type(scope): description`.

#### 7. Open a Pull Request

1. Go to your fork on GitHub
2. Click "Compare & pull request"
3. Fill in the PR template:
   - **Title**: Clear description of the change
   - **Description**: What you changed, why, and how to test it
   - **Screenshots**: If the change affects the UI
4. Link the related issue (e.g., "Closes #42")
5. Click "Create pull request"

The CI will automatically run all checks. If any fail, see the [Troubleshooting](#troubleshooting) section or ask for help in the PR comments.

### Tips for Success

- **Start small** — Documentation fixes, typo corrections, or adding comments are great first contributions
- **Read existing code** — Look at recently merged PRs to understand the project style
- **Ask questions** — Open an issue or comment on the PR if you are unsure about anything
- **Be patient** — Reviews may take a few days; maintainers are volunteers
- **One PR per feature** — Keep pull requests focused and easy to review

## Project Structure

```
fuudelivery/
├── Backend/                  # Go microservices (legacy)
│   ├── auth_api/             # Authentication service
│   ├── orders_api/           # Orders service
│   ├── delivery_api/         # Delivery tracking
│   ├── payment_api/          # Payment processing
│   ├── chat_api/             # Real-time chat
│   └── Payment/              # Payment service (standalone)
├── cmd/fuudelivery/          # Main Go monolith
├── Frontend/
│   ├── AppComida/            # React Native (Customer app)
│   ├── AppEntrega/           # React Native (Delivery app)
│   ├── WebRestaurant/        # React + Tailwind (Restaurant dashboard)
│   ├── WebAdmin/             # React (Admin panel)
│   └── PaymentPanel/         # HTML/JS (Payment approval)
├── scripts/                  # Build and utility scripts
└── references/               # Documentation
```

## Coding Conventions

### Go (Backend)

- **Formatter**: Run `gofmt -s` on all files before committing
- **Linter**: Run `go vet ./...` to catch common issues
- **Naming**: Follow Go conventions (PascalCase for exported, camelCase for unexported)
- **Error handling**: Always check and handle errors explicitly
- **Comments**: Add comments for exported functions and complex logic
- **Packages**: Keep packages focused and avoid circular dependencies

```go
// Good
func GetOrderByID(id string) (*Order, error) {
    if id == "" {
        return nil, errors.New("order ID is required")
    }
    // Implementation...
}

// Bad - missing error handling
func GetOrderByID(id string) *Order {
    return &Order{ID: id}
}
```

### TypeScript/JavaScript (Frontend)

- **Formatter**: Use Prettier with default settings
- **Linting**: Follow ESLint rules configured in each project
- **Components**: Use functional components with hooks
- **State**: Prefer local state; use context for shared state
- **Styling**: Use Tailwind CSS for web apps, StyleSheet for React Native

```tsx
// Good
interface Props {
  title: string;
  onSelect: (id: string) => void;
}

const MenuItem: React.FC<Props> = ({ title, onSelect }) => {
  const [isSelected, setIsSelected] = useState(false);
  
  return (
    <button onClick={() => onSelect(title)}>
      {title}
    </button>
  );
};
```

### General

- **Files**: One component/function per file
- **Imports**: Group imports (external, internal, relative)
- **No secrets**: Never commit API keys, passwords, or credentials
- **Documentation**: Update README.md for significant changes

## Testing

### Running Tests

```bash
# Go tests (all modules)
go test ./...

# Go tests (specific module)
cd cmd/fuudelivery && go test ./...

# Go tests with verbose output
go test -v ./...

# Integration tests (requires Docker)
cd cmd/fuudelivery && go test -tags=integration -v -run 'TestFullFlow|TestErrorScenarios|TestAdminBootstrap' ./
cd Backend/payment_api && go test -tags=integration -v -run 'TestCheckoutE2E' ./app/handlers/

# Frontend tests
cd Frontend/WebRestaurant && npm test
```

### Writing Tests

#### Go Tests

```go
func TestGetOrder(t *testing.T) {
    // Arrange
    order := &Order{ID: "123", Status: "pending"}
    
    // Act
    result := processOrder(order)
    
    // Assert
    if result.Status != "confirmed" {
        t.Errorf("Expected confirmed, got %s", result.Status)
    }
}
```

#### React Tests

```tsx
import { render, screen } from '@testing-library/react';
import MenuItem from './MenuItem';

test('renders menu item title', () => {
  render(<MenuItem title="Pizza" onSelect={() => {}} />);
  expect(screen.getByText('Pizza')).toBeInTheDocument();
});
```

### Test Coverage

- **Backend**: Aim for >70% coverage on new code
- **Frontend**: Test critical user interactions
- **Integration**: Test API endpoints with real database (Docker)

## CI Pipeline

Every push to `master` and every pull request triggers the **CI Gate** workflow (`.github/workflows/ci.yml`). Here is what runs and how to reproduce each check locally.

### Checks Overview

| Check | Scope | Command | Time |
|-------|-------|---------|------|
| **Go Build** | 7 modules (matrix) | `go build ./...` | ~30s |
| **Go Vet** | 7 modules (matrix) | `go vet ./...` | ~10s |
| **Go Test** | 7 modules (matrix) | `go test ./... -count=1 -timeout 60s` | ~60s |
| **gofmt** | All Go code | `gofmt -l -s Backend/ cmd/` | ~5s |
| **govulncheck** | 7 modules (matrix) | `govulncheck ./...` | ~30s |
| **npm test** | WebRestaurant | `npm test -- --watchAll=false` | ~30s |
| **npm audit** | WebRestaurant, WebAdmin, PaymentPanel | `npm audit --audit-level=moderate` | ~10s |

### Go Modules Tested

The CI runs the same 4 checks (build, vet, test, govulncheck) against each module in a matrix:

- `cmd/fuudelivery` — Main monolith
- `Backend/auth_api` — Authentication
- `Backend/payment_api` — Payment gateway
- `Backend/orders_api` — Orders
- `Backend/delivery_api` — Delivery tracking
- `Backend/chat_api` — Chat

### Running CI Checks Locally

Before pushing, run the same checks CI will run:

```bash
# Go checks (run from project root)
cd cmd/fuudelivery && go mod tidy && go build ./... && go vet ./... && go test ./... -count=1 -timeout 60s && cd ../..

# Format check
gofmt -l -s Backend/ cmd/
# If any files listed, format them:
gofmt -s -w Backend/ cmd/

# Vulnerability scan (install once)
go install golang.org/x/vuln/cmd/govulncheck@latest
cd cmd/fuudelivery && govulncheck ./... && cd ../..

# Frontend checks
cd Frontend/WebRestaurant && npm install && npm test -- --watchAll=false && npm run build && cd ../..

# NPM audit
cd Frontend/WebRestaurant && npm audit --audit-level=moderate && cd ../..
cd Frontend/WebAdmin && npm audit --audit-level=moderate && cd ../..
# PaymentPanel arquivado em legacy/ — não passa mais por npm audit
```

### Why CI Might Fail

| Failure | Cause | Fix |
|---------|-------|-----|
| `gofmt` lists files | Code not formatted | Run `gofmt -s -w .` in the affected module |
| `go mod tidy` changes go.sum | Stale dependencies | Commit the updated go.sum |
| `go vet` warnings | Suspicious code patterns | Read the warning and fix the issue |
| `go test` timeout | Test exceeds 60s or hangs | Check for infinite loops or slow DB calls |
| `govulncheck` | Known vulnerability in dependency | Update the dependency with `go get` |
| `npm audit` moderate+ | Vulnerable npm package | Run `npm audit fix` or update the package |
| `npm test` fails | React component or logic error | Run `npm test` locally and fix the failing test |

## Payment System (arquivado)

> ⚠️ O serviço separado `Backend/Payment` (FuuPayment) foi **arquivado e removido**
> do repositório. Todos os fluxos de pagamento — PIX/cartão (AbacatePay), carteiras,
> chargebacks, aprovações, split — vivem hoje em `Backend/payment_api`, **embutido
> no monolito** (`cmd/fuudelivery`). Não edite nem procure código em `Backend/Payment`.

## Pull Request Process

### 1. Create a Branch

```bash
# Feature branch
git checkout -b feature/your-feature-name

# Bug fix branch
git checkout -b fix/your-bug-fix

# Documentation branch
git checkout -b docs/your-documentation
```

### 2. Make Changes

- Follow coding conventions above
- Write tests for new functionality
- Update documentation if needed

### 3. Run Checks Locally

```bash
# Go
cd cmd/fuudelivery
gofmt -s -w .
go vet ./...
go test ./...

# Frontend
cd Frontend/WebRestaurant
npm test
npm run build
```

### 4. Commit Changes

```bash
git add .
git commit -m "feat(orders): add order status validation"
```

See [Commit Message Guidelines](#commit-message-guidelines) below.

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub with:
- **Title**: Clear, concise description
- **Description**: What changed and why
- **Screenshots**: For UI changes
- **Related Issues**: Link to any related issues

### 6. Code Review

- Address review comments
- Make requested changes
- Re-request review when ready

### 7. Merge

Once approved, Squash and Merge into `master`.

## Commit Message Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting (no code change) |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf` | Performance improvement |
| `test` | Adding or correcting tests |
| `chore` | Build process or auxiliary tool changes |
| `ci` | CI configuration changes |

### Examples

```bash
# Feature
git commit -m "feat(payments): add PIX payment support"

# Bug fix
git commit -m "fix(orders): prevent duplicate order submission"

# Documentation
git commit -m "docs(readme): add environment variables table"

# Breaking change
git commit -m "feat(api)!: change authentication endpoint response format

BREAKING CHANGE: /auth/login now returns { token, user } instead of { access_token }"
```

### Scope

Common scopes for this project:
- `orders` - Orders functionality
- `payments` - Payment processing
- `delivery` - Delivery tracking
- `auth` - Authentication
- `chat` - Real-time chat
- `api` - API changes
- `ui` - UI changes
- `docs` - Documentation
- `ci` - CI/CD

## Troubleshooting

Common issues new contributors encounter and how to fix them.

### MongoDB Connection Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `connection refused` | MongoDB not running | Start with Docker: `docker run -d --name mongodb -p 27017:27017 mongo:6` |
| `server selection timeout` | Wrong `MONGO_URI` | Check `MONGO_URI` env var matches your Docker port |
| `authentication failed` | Credentials mismatch | If using Docker with auth, match username/password in `MONGO_URI` |

```bash
# Verify MongoDB is running
docker ps | grep mongodb

# Test connection
mongosh "mongodb://localhost:27017" --eval "db.runCommand({ ping: 1 })"

# Check logs
docker logs mongodb --tail 20
```

### Redis Timeout

| Error | Cause | Fix |
|-------|-------|-----|
| `dial tcp: connect: connection refused` | Redis not running | Start with Docker: `docker run -d --name redis -p 6379:6379 redis:7` |
| `context deadline exceeded` | Redis overloaded or network issue | Check `REDIS_URL` env var, restart Redis |
| Queue messages not processing | `REDIS_URL` not set | The monolith falls back to Go channels — set `REDIS_URL` for production |

```bash
# Verify Redis is running
docker ps | grep redis

# Test connection
redis-cli ping

# Check queue depth (stream length)
redis-cli XLEN queue:payments
```

> **Note:** If `REDIS_URL` is not configured, the monolith uses in-memory Go channels as a fallback. This works for local development but messages are lost on restart.

### JWT Secret Mismatch

| Error | Cause | Fix |
|-------|-------|-----|
| `token is expired` | Clock skew or old token | Clear browser storage and re-login |
| `signature is invalid` | Different `JWT_SECRET` between services | Ensure Payment Service and Monolith use the **same** `JWT_SECRET` |
| `crypto/rsa: decryption error` | Wrong algorithm | Check that `JWT_SECRET` is a plain string, not a PEM key |

```bash
# Verify both services use the same secret
echo "Monolith: $JWT_SECRET"
echo "Payment: $JWT_SECRET"  # Must match!
```

### CORS Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `Access-Control-Allow-Origin` missing | Origin not in `ALLOWED_ORIGINS` | Add your origin to `ALLOWED_ORIGINS` env var |
| `preflight response invalid` | OPTIONS request blocked | Ensure CORS middleware is configured for all routes |
| Works on localhost, fails in production | Origin mismatch | Production domains must be in `ALLOWED_ORIGINS` (comma-separated) |

```bash
# Example ALLOWED_ORIGINS
ALLOWED_ORIGINS=http://localhost:3000,https://fuudelivery-admin-lv7f.onrender.com
```

### AbacatePay Webhook Failures

| Error | Cause | Fix |
|-------|-------|-----|
| `invalid webhook signature` | Wrong `ABACATE_PAY_WEBHOOK_SECRET` | Ensure the secret matches the AbacatePay dashboard |
| `webhook not received` | Network/firewall issue | Check Render logs, verify webhook URL in AbacatePay dashboard |
| `payment not processing` | Redis queue full | Check Redis queue depth, restart Payment Service consumer |

```bash
# Test webhook endpoint
curl -X POST http://localhost:8084/webhook   -H "Content-Type: application/json"   -d '{"event":"payment.paid","data":{}}'

# Check Payment Service logs
curl http://localhost:8084/health
```

### Port Conflicts

| Error | Cause | Fix |
|-------|-------|-----|
| `bind: address already in use` | Port occupied by another process | Kill the process or change the `PORT` env var |
| Monolith and Payment both default to 3000 | Port collision | Set different `PORT` values (e.g., 3000 for monolith, 8084 for payment) |

```bash
# Find process using a port
lsof -i :3000

# Kill it
kill -9 <PID>

# Or use a different port
PORT=3001 go run main.go
```

### `go build` Failures

| Error | Cause | Fix |
|-------|-------|-----|
| `cannot find module` | Missing dependency | Run `go mod tidy` in the module directory |
| `go.sum` mismatch | Stale checksums | Run `go mod tidy` to regenerate |
| `undefined: ...` | Import path changed | Check if the package was moved or renamed |

### Frontend Build Failures

| Error | Cause | Fix |
|-------|-------|-----|
| `Module not found: Can't resolve ...` | Missing npm package | Run `npm install` |
| `Peer dependency conflict` | Version mismatch | Use `npm install --legacy-peer-deps` |
| `expo-module-gradle-plugin not found` | Bad config plugin in `app.json` | Remove `withGradleWorkaround` from plugins, keep only `expo-router` |
| `npm install` hangs | Windows Defender / slow I/O | Disable real-time protection temporarily or use `yarn` |

## Code Review Checklist

### For Authors

Before requesting review:
- [ ] Code follows project conventions
- [ ] Tests pass locally
- [ ] No console.log or debug statements
- [ ] No hardcoded secrets or credentials
