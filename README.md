# 🚀 Zenith

## 💫 What is Zenith?

Zenith transforms the deployment experience by enabling instant web application deployment from GitHub repositories with a single click. No more complex configurations or deployment headaches—just push your code and go live in seconds.

## 🌟 Key Features

- **One-Click Deployment** - From repository to live site in seconds
- **Framework Agnostic** - Support for React, Vue, Angular, and other popular frameworks
- **Automated Build Detection** - Intelligent detection of project structure and build requirements
- **Instant Public URLs** - Every deployment gets a shareable public URL via ngrok
- **Modern Dashboard** - Clean interface for managing all your deployments

## 🏗️ Architecture

Zenith's modular microservice architecture ensures reliability and scalability:

### Core Services

| Service | Description |
|---------|-------------|
| **Request Handler** | Orchestrates the deployment pipeline and serves the frontend UI |
| **Upload Service** | Handles repository cloning and cloud storage integration |
| **Build Service** | Manages build processes for different application types |

## 🚦 Getting Started

### Prerequisites

- Go 1.16+
- Node.js 14+
- npm or yarn
- ngrok account
- Backblaze B2 account (or compatible S3 storage)

### Environment Setup

Create `.env` files in each service directory with the following variables:

```
GITHUB_TOKEN=your_github_token
B2_ACCESS_KEY=your_b2_access_key
B2_SECRET_KEY=your_b2_secret_key
B2_BUCKET=your_bucket_name
B2_ENDPOINT=your_b2_endpoint
B2_REGION=your_b2_region
```

### Quick Start

1. **Start the Upload Service**
   ```bash
   cd upload_service
   go run main.go
   ```

2. **Start the Build Service**
   ```bash
   cd build_service
   go run main.go
   ```

3. **Start the Request Handler**
   ```bash
   cd request_handler
   go run main.go
   ```

4. **Launch the Frontend (optional)**
   ```bash
   cd frontend
   npm run dev
   ```

## 🔌 API Reference

### Request Handler (port 8080)

**Deploy a repository (query parameter)**
```
GET /deploy?url=<github_repo_url>
```

**Deploy a repository (JSON body)**
```
POST /deploy
Content-Type: application/json

{
  "url": "https://github.com/username/repository"
}
```

### Upload Service (port 8081)

**Clone and upload a repository**
```
POST /upload
Content-Type: application/json

{
  "url": "https://github.com/username/repository"
}
```

### Build Service (port 8082)

**Build a repository**
```
POST /build
Content-Type: application/json

{
  "repo": "repository-name",
  "use_template": true,
  "template": "create-react-app"
}
```

## 🎬 Usage Example

1. Visit the dashboard at http://localhost:3000
2. Enter a GitHub repository URL (e.g., https://github.com/username/react-app)
3. Click "Deploy"
4. Watch the live deployment progress
5. Access your deployed site via the provided public URL

## 🤝 Contributing

Contributions make the open-source community an amazing place to learn, inspire, and create. Any contributions to Zenith are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.

## 📬 Contact

Project Link: [https://github.com/yourusername/zenith](https://github.com/deltacoder2603/zenith)

---


    Built with ❤️ by the Zenith Team
