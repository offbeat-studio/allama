# Allama

Allama is a versatile API router and provider management system designed to aggregate and manage multiple AI model providers under a unified interface. It supports integration with various providers like OpenAI, Anthropic, and Ollama, allowing seamless access to different AI models through a single API endpoint. This project aims to simplify the interaction with multiple AI services by providing a consistent interface and handling the complexity of provider-specific configurations and communications.

## Features

- **Unified API**: Access multiple AI providers through a single RESTful API.
- **Provider Management**: Easily configure and manage different AI service providers.
- **Model Aggregation**: List and interact with models from various providers as if they were hosted locally.
- **Database Integration**: Store provider and model information persistently for quick access and management.

## Installation

To get started with Allama, you can clone the repository and build the application locally or use the provided Docker setup for a containerized environment.

### Prerequisites

- Go 1.18 or higher (for building from source)
- Docker and Docker Compose (for containerized setup)
- Git (for cloning the repository)

### Clone the Repository

```bash
git clone https://github.com/offbeat-studio/allama.git
cd allama
```

### Running with Docker

The easiest way to run Allama is using Docker Compose, which sets up the application and its dependencies in isolated containers.

1. Ensure Docker and Docker Compose are installed on your system.
2. Copy the `src/example.env` file to `src/.env` and update the environment variables as needed (e.g., API keys for providers).
   ```bash
   cp src/example.env src/.env
   ```
3. From the root of the repository, run:
   ```bash
   docker-compose -f Docker/docker-compose.yml up --build
   ```
4. Allama will be accessible at `http://localhost:8080`.

### Building and Running Locally

If you prefer to build and run Allama directly on your system:

1. Ensure Go is installed on your system.
2. Copy the `src/example.env` file to `src/.env` and update the environment variables as needed.
   ```bash
   cp src/example.env src/.env
   ```
3. From the `src` directory, build and run the application:
   ```bash
   cd src
   go mod tidy
   go run main.go
   ```
4. Allama will be accessible at `http://localhost:8080` (or the port specified in your configuration).

## Usage

Once Allama is running, you can interact with it through its API endpoints. Here are some basic operations:

- **List Models**: Retrieve a list of available models from all configured providers.
  ```bash
  curl http://localhost:8080/api/v1/models
  ```
- **Chat Completions**: Send chat messages to a specific model.
  ```bash
  curl -X POST http://localhost:8080/api/v1/chat/completions \
       -H "Content-Type: application/json" \
       -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello, how are you?"}]}'
  ```

For compatibility with Ollama clients, Allama also supports Ollama-specific endpoints:
- **List Tags**: Retrieve model tags as if querying an Ollama server.
  ```bash
  curl http://localhost:8080/api/tags
  ```

## Configuration

Allama uses environment variables for configuration. You can set these in a `.env` file in the `src` directory or directly in your environment. Key variables include:

- `PORT`: The port on which the Allama server runs (default: 8080).
- `DATABASE_PATH`: Path to the SQLite database file for storing provider and model data.
- Provider-specific variables like `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, and enable flags like `OPENAI_ENABLED=true`.

## Contributing

We welcome contributions to Allama! Whether it's bug fixes, new features, or documentation improvements, your help is appreciated. Here's how you can contribute:

### Reporting Issues

If you encounter any bugs or have feature requests, please open an issue on the GitHub repository. Provide as much detail as possible, including steps to reproduce the issue, expected behavior, and actual behavior.

### Submitting Pull Requests

1. **Fork the Repository**: If you're not a core contributor, fork the Allama repository to your own GitHub account.
2. **Clone the Repository**: Clone the repository to your local machine.
   ```bash
   git clone https://github.com/offbeat-studio/allama.git
   cd allama
   ```
3. **Create a Branch**: Create a new branch for your changes with a descriptive name.
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. **Make Changes**: Implement your changes, ensuring to follow the coding style and conventions used in the project.
5. **Test Your Changes**: Make sure your changes do not break existing functionality and add tests if applicable.
6. **Commit Your Changes**: Write clear, concise commit messages that describe the purpose of your changes.
   ```bash
   git commit -m "Add feature: description of your feature"
   ```
7. **Push to GitHub**: Push your branch to the repository.
   ```bash
   git push origin feature/your-feature-name
   ```
8. **Open a Pull Request**: Go to the GitHub repository page, switch to your branch, and click "New Pull Request". Fill out the PR template with details about your changes, referencing any related issues.

### Code Style

- Follow Go best practices and conventions.
- Use meaningful variable and function names.
- Comment your code where necessary to explain complex logic.
- Ensure your code is formatted with `go fmt`.

### Development Setup

To set up the development environment:

1. Install Go and necessary tools.
2. Clone the repository and navigate to the `src` directory.
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Run tests to ensure everything is working:
   ```bash
   go test ./...
   ```

## License

Allama is open-source software released under the [MIT License](LICENSE). You are free to use, modify, and distribute this software as long as you include the original copyright and license notice in any copy of the software/source.

## Contact

For questions or further information, you can reach out to the maintainers via GitHub issues or through the contact details provided in the repository.

Thank you for using and contributing to Allama!
