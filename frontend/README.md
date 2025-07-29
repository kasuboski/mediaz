# Mediaz Frontend

Modern React-based web interface for the Mediaz media management platform, providing an intuitive interface for discovering, organizing, and managing your movie and TV show collections.

## UI Overview

The frontend provides a comprehensive media management experience:

- **Discover Page**: Browse and search for new movies and TV shows using TMDB integration
- **Movies Library**: View and manage your indexed movie collection with detailed metadata
- **Series Library**: Organize and browse your TV show episodes and seasons
- **Media Details**: Rich detail pages with poster art, metadata, cast information, and download management
- **Request System**: Submit media requests for automated downloading through configured indexers
- **Responsive Design**: Fully responsive layout optimized for desktop and mobile devices

## Technologies Used

### Core Framework & Libraries
- **React 18** - Modern React with hooks and concurrent features
- **TypeScript** - Full type safety throughout the application
- **Vite** - Fast build tool and development server
- **React Router** - Client-side routing and navigation
- **TanStack Query** - Server state management and caching

### UI Components & Styling  
- **shadcn/ui** - High-quality accessible component library built on Radix UI
- **Radix UI** - Unstyled, accessible UI primitives
- **Tailwind CSS** - Utility-first CSS framework for rapid styling
- **Lucide React** - Beautiful icon library
- **Sonner** - Toast notifications

### Form Handling & Validation
- **React Hook Form** - Performant forms with minimal re-renders
- **Zod** - TypeScript-first schema validation

### Additional Features
- **next-themes** - Dark/light theme support
- **date-fns** - Modern date utility library
- **Recharts** - Composable charting library

## Developer Setup

### Prerequisites

This project uses **direnv** with **Nix flake** for reproducible development environments:

1. **Install direnv**: Follow instructions at [direnv.net](https://direnv.net/docs/installation.html)
2. **Install Nix**: Follow instructions at [nixos.org](https://nixos.org/download.html)
3. **Enable flakes**: Add `experimental-features = nix-command flakes` to your nix configuration

### Environment Setup

The project root contains a `flake.nix` that provides:
- Go 1.24 for backend development
- Node.js for frontend development
- All required development tools (golangci-lint, mockgen, etc.)

```bash
# Navigate to project root and allow direnv
cd /path/to/mediaz
direnv allow

# The flake will automatically install all dependencies
# including Node.js, Go, and development tools
```

### Frontend Development

```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
npm install

# Start development server (runs on port 3000)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Run linting
npm run lint
```

### Backend Integration

The frontend expects the Mediaz backend API server running on `localhost:8080`. Start the backend server with:

```bash
# From project root
go build -o mediaz main.go
./mediaz serve
```

### Development Workflow

1. Ensure backend is running on port 8080
2. Start frontend dev server: `npm run dev`
3. Frontend will be available at `http://localhost:3000`
4. API calls are proxied to the backend automatically

### Project Structure

```
src/
├── components/          # Reusable UI components
│   ├── layout/         # Layout components (sidebar, header)
│   ├── media/          # Media-specific components
│   └── ui/             # shadcn/ui component library
├── hooks/              # Custom React hooks
├── lib/                # Utilities and API client
├── pages/              # Route components
└── main.tsx           # Application entry point
```
