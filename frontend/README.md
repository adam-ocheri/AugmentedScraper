# Frontend - Content Analysis System

This is the frontend application for the Content Analysis System, built with Next.js and React.

## Features

- Submit URLs for content analysis
- Real-time status updates via WebSocket
- Task history and results display
- Modern, responsive UI with Tailwind CSS
- Docker support for both development and production

## Development

### Prerequisites

- Node.js 18 or higher
- Docker and Docker Compose (for containerized development)

### Local Development

1. Install dependencies:

   ```bash
   npm install
   ```

2. Start the development server:

   ```bash
   npm run dev
   ```

3. Open [http://localhost:3000](http://localhost:3000) in your browser.

### Docker Development

1. Start all services in development mode:

   ```bash
   docker-compose -f docker-compose.dev.yml up --build
   ```

2. The frontend will be available at [http://localhost:3000](http://localhost:3000)

### Production

1. Build and start all services:

   ```bash
   docker-compose up --build
   ```

2. The frontend will be available at [http://localhost:3000](http://localhost:3000)

## Environment Variables

- `NEXT_PUBLIC_API_URL`: Backend API URL (default: http://localhost:8080)
- `NEXT_PUBLIC_WS_URL`: WebSocket URL (default: ws://localhost:8080)

## Project Structure

```
frontend/
├── src/
│   └── app/
│       ├── page.tsx          # Main application page
│       ├── layout.tsx        # Root layout
│       └── globals.css       # Global styles
├── public/                   # Static assets
├── Dockerfile               # Production Dockerfile
├── Dockerfile.dev           # Development Dockerfile
└── package.json             # Dependencies and scripts
```

## Available Scripts

- `npm run dev`: Start development server
- `npm run build`: Build for production
- `npm run start`: Start production server
- `npm run lint`: Run ESLint

## API Integration

The frontend communicates with the backend through:

1. **HTTP API**: For submitting URLs and getting initial responses
2. **WebSocket**: For real-time status updates and task completion notifications

### API Endpoints

- `POST /submit`: Submit a URL for analysis
- `GET /status/{uuid}`: Get task status (if needed)
- `WS /ws`: WebSocket connection for real-time updates

## WebSocket Message Format

The frontend expects WebSocket messages in the following format:

```json
{
  "type": "task_update",
  "uuid": "task-uuid",
  "status": "SUCCESS|FAILED|WORKING",
  "summary": "Article summary...",
  "sentiment": "Positive/Negative/Neutral"
}
```
