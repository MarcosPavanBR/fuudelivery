# Contributing to FuuDelivery

Thank you for considering contributing to FuuDelivery! This guide will help you get started.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Coding Conventions](#coding-conventions)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Commit Message Guidelines](#commit-message-guidelines)
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

# Payment Panel (HTML/JS)
# Just open Frontend/PaymentPanel/index.html in your browser

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
cd Backend/Payment && go test -tags=integration ./... -v

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

## Payment System (FuuPayment)

The payment system is a standalone Go service (`Backend/Payment`) that handles payment processing, digital wallets, chargebacks, and risk scoring. It communicates with the monolith (`cmd/fuudelivery`) via Redis queues.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    MONOLITH (cmd/fuudelivery)                       │
│                    Go 1.23 + Fiber v2                               │
│                                                                     │
│  payment_api/webhook.go ──LPush──▶ queue:payments                   │
│                                    │                                │
│                                    │  Redis LPush/BRPop             │
│                                    ▼                                │
│  cmd/fuudelivery/main.go ◀──Subscribe── queue:payment_updates       │
└─────────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
┌───────────────────────┐  ┌───────────────────────────────────────────┐
│   PAYMENT SERVICE     │  │           INFRAESTRUTURA                   │
│   (Backend/Payment)   │  │                                           │
│                       │  │  MongoDB — pagamentos, carteiras,         │
│   consumers/          │  │               estornos                    │
│     └─ BRPop ────────┼──│  Redis — fila + cache                     │
│                       │  │  AbacatePay — gateway PIX/Cartão          │
│   services/           │  │                                           │
│     └─ approval_engine│  └───────────────────────────────────────────┘
│     └─ wallet_service │
│     └─ risk_scorer    │
└───────────────────────┘
```

### Communication Flow

1. **Payment API** (`payment_api/webhook.go`) receives webhook from AbacatePay
2. Publishes payment confirmation to `queue:payments` via Redis LPush
3. **Payment Service** (`consumers/payment_consumer.go`) consumes from `queue:payments` via Redis BRPop
4. Processes payment: updates MongoDB, credits wallet, checks risk score
5. Publishes status update to `queue:payment_updates` via Redis LPush
6. **Monolith** (`cmd/fuudelivery/main.go`) subscribes to `queue:payment_updates` and updates order status

### Running Locally

```bash
# Start MongoDB
docker run -d --name mongodb -p 27017:27017 mongo:6

# Start Redis (optional — uses Go channels fallback if not available)
docker run -d --name redis -p 6379:6379 redis:7

# Start Payment Service
cd Backend/Payment
go mod tidy
go run main.go

# Start Monolith (in separate terminal)
cmd/fuudelivery
go run main.go
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `MONGO_URI` | MongoDB connection string | Yes |
| `JWT_SECRET` | JWT signing secret (must match monolith) | Yes |
| `REDIS_URL` | Redis URL for queue communication | Recommended |
| `ABACATE_PAY_API_KEY` | AbacatePay API key | Yes (production) |
| `ABACATE_PAY_WEBHOOK_SECRET` | Webhook secret for signature verification | Yes (production) |

### Testing Payment Flows

```bash
# Run unit tests
cd Backend/Payment
go test ./...

# Run integration tests (requires Docker)
go test -tags=integration ./... -v

# Run specific test suite
go test ./services/... -v           # Wallet, chargeback, approval tests
go test ./consumers/... -v         # Redis consumer tests
```

### Key Concepts

- **Risk Scoring**: 4 factors (amount, frequency, establishment history, chargeback history)
- **Wallet Operations**: Atomic `$inc` operations on MongoDB to prevent race conditions
- **Split Payment**: 5% platform, 85% restaurant, delivery fee to driver
- **Idempotency**: Transaction deduplication via `TransactionExistsByReference`

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

## Code Review Checklist

### For Authors

Before requesting review:
- [ ] Code follows project conventions
- [ ] Tests pass locally
- [ ] No console.log or debug statements
- [ ] No hardcoded secrets or credentials
