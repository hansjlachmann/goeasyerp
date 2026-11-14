# GoEasyERP Web Frontend

Modern, fast, keyboard-driven web frontend for GoEasyERP, built with SvelteKit.

## Features

✅ **SvelteKit** - Fast, modern framework with SSR support
✅ **TailwindCSS** - Utility-first CSS with a classic ERP theme
✅ **TypeScript** - Type-safe development
✅ **Keyboard Shortcuts** - keyboard navigation (Alt+N, Ctrl+E, etc.)
✅ **API Client** - Type-safe API service for backend communication
✅ **Session Management** - Centralized session/state management
✅ **Component Library** - Reusable UI components (Button, Card, Input, etc.)

## Project Structure

```
web/
├── src/
│   ├── lib/
│   │   ├── components/      # Reusable UI components
│   │   │   ├── Button.svelte
│   │   │   ├── Card.svelte
│   │   │   ├── Input.svelte
│   │   │   └── PageHeader.svelte
│   │   ├── stores/          # Svelte stores for state management
│   │   │   └── session.ts   # Session store (user, company, language)
│   │   ├── services/        # API clients and services
│   │   │   └── api.ts       # REST API client
│   │   ├── types/           # TypeScript type definitions
│   │   │   ├── api.ts       # API types
│   │   │   └── session.ts   # Session types
│   │   └── utils/           # Utility functions
│   │       ├── shortcuts.ts # Keyboard shortcut system
│   │       └── cn.ts        # Tailwind class merger
│   ├── routes/              # SvelteKit routes (pages)
│   │   ├── +layout.svelte   # Root layout
│   │   ├── +page.svelte     # Home page
│   │   └── customers/       # Customer list page
│   │       └── +page.svelte
│   ├── app.html             # HTML entry point
│   └── app.css              # Global styles (Tailwind)
├── static/                  # Static assets
├── package.json
├── svelte.config.js
├── vite.config.js
├── tailwind.config.js
└── tsconfig.json
```

## Installation

### Prerequisites

- Node.js 18+ and npm (or yarn/pnpm)
- Go backend API server running on port 8080

### Setup

```bash
cd web
npm install
```

## Development

Start the development server:

```bash
npm run dev
```

The app will be available at `http://localhost:5173`

The Vite dev server will proxy API requests to the Go backend:
- `/api/*` → `http://localhost:8080/api/*`
- `/ws/*` → `ws://localhost:8080/ws/*`

## Building for Production

```bash
npm run build
```

The built application will be in `build/` directory, ready to deploy as a Node.js server.

Preview the production build:

```bash
npm run preview
```

## Keyboard Shortcuts

keyboard shortcuts are implemented throughout the application:

| Shortcut       | Action                 |
| -------------- | ---------------------- |
| `Alt+N`        | New record             |
| `Ctrl+E`       | Edit record            |
| `Ctrl+D`       | Delete record          |
| `Ctrl+S`       | Save                   |
| `Ctrl+F`       | Find / Filter          |
| `F5`           | Refresh                |
| `Ctrl+Home`    | First record           |
| `Ctrl+End`     | Last record            |
| `PageUp/Down`  | Previous/Next record   |
| `↑/↓`          | Navigate table rows    |
| `Enter`        | Open/Edit selected row |
| `Escape`       | Cancel                 |
| `Ctrl+O`       | Switch company         |
| `Enter`        | Next field (card page) |

## API Client Usage

### Using the generic API client:

```typescript
import { api } from '$services/api';

// List records
const customers = await api.listRecords('Customer', {
  filters: [{ field: 'city', operator: 'eq', value: 'Oslo' }],
  sort_by: 'no',
  sort_order: 'asc',
  page: 1,
  page_size: 50
});

// Get single record
const customer = await api.getRecord('Customer', 'CUST-001');

// Insert
await api.insertRecord('Customer', { no: 'CUST-002', name: 'New Customer' });

// Modify
await api.modifyRecord('Customer', 'CUST-001', { name: 'Updated Name' });

// Delete
await api.deleteRecord('Customer', 'CUST-001');

// Validate field
const result = await api.validateField('Customer', 'payment_terms_code', '30DAYS');
```

### Using typed API clients:

```typescript
import { customerApi, paymentTermsApi } from '$services/api';

const customers = await customerApi.list();
const customer = await customerApi.get('CUST-001');
await customerApi.insert({ no: 'CUST-002', name: 'New' });
```

## Creating a New Page

### 1. Create the route file:

```svelte
<!-- src/routes/my-page/+page.svelte -->
<script lang="ts">
  import PageHeader from '$components/PageHeader.svelte';
  import Card from '$components/Card.svelte';
  import { shortcuts, createShortcutMap } from '$utils/shortcuts';

  // Your page logic here
  const shortcutMap = createShortcutMap({
    onNew: () => console.log('New'),
    onRefresh: () => console.log('Refresh')
  });
</script>

<div use:shortcuts={shortcutMap}>
  <PageHeader title="My Page" />
  <Card>
    <!-- Your content -->
  </Card>
</div>
```

### 2. Using components:

```svelte
<script>
  import Button from '$components/Button.svelte';
  import Input from '$components/Input.svelte';
</script>

<Button variant="primary" on:click={() => alert('Clicked')}>
  Save
</Button>

<Input
  label="Customer Name"
  bind:value={customerName}
  error={errors.name}
/>
```

## Session Management

Access session state anywhere:

```svelte
<script>
  import { session, currentLanguage, isAuthenticated } from '$stores/session';
</script>

<p>Company: {$session.company}</p>
<p>Language: {$currentLanguage}</p>
{#if $isAuthenticated}
  <p>Welcome, {$session.userName}!</p>
{/if}
```

## Styling

### Using Tailwind utility classes:

```svelte
<div class="bg-surface rounded-lg shadow-sm p-6">
  <h1 class="text-2xl font-bold text-heading">Title</h1>
</div>
```

### Using predefined component classes:

```svelte
<button class="btn btn-primary">Primary Button</button>
<input class="input" placeholder="Enter text..." />
<div class="card">
  <div class="card-header">Header</div>
  <div class="card-body">Content</div>
</div>
```

### Colours:

Every colour is a theme variable (`src/app.css`), so it follows the user's theme and dark mode:

- `bg-surface` - panels, cards, dropdowns (never `bg-white`)
- `text-heading` - headings; `text-fg` - body text
- `gray-*` - the slate/grey ramp (50-900)
- `primary-*` / `blue-*` - the accent ramp, amber by default (main actions, highlights)

## Docker/DevContainer Support

The frontend is configured to run in Docker with hot-reload:

```yaml
# docker-compose.yml (see root directory)
frontend:
  ports:
    - "5173:5173"
  environment:
    - VITE_API_URL=http://backend:8080
  command: npm run dev -- --host
```

## Type Safety

All API responses and data structures are typed:

```typescript
import type { Customer, PaymentTerms } from '$types/api';

const customer: Customer = {
  no: 'CUST-001',
  name: 'Adventure Works',
  city: 'Oslo'
  // TypeScript will enforce all fields
};
```

## Next Steps

The Go API backend, YAML page definitions, and the generic `PageRenderer` are already
implemented. Remaining frontend work (WebSocket real-time updates, additional page types)
is tracked centrally in [`/TODO.md`](../TODO.md).

## License

MIT
