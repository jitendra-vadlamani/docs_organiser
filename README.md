# MLX-Powered File Mover

A standalone Go application that uses a local MLX AI model (via HTTP) to intelligently categorize and move documents.

## Features
- 🤖 **Multi-Provider AI Engine**: Support for any OpenAI-compatible API (MLX, Ollama, Llama.cpp).
- 🔄 **Model Pool Management**: Set a **Default Model** or dynamically switch between multiple providers.
- 🚀 **GPU-Accelerated**: Optimized for Apple Silicon via MLX and local Ollama instances.
- **Intelligent Categorization**: Uses Llama-3 models to analyze and organize your messy documents.
- **Production Ready Dashboard**: Glassmorphic UI with live throughput charts, metrics, and configuration management.
- **Smart Extraction**: Optimized for PDF and plain text documents.

## 1. Starting the AI Server

Before running the Go application, you must have an MLX server running locally.

### Prerequisites
- Apple Silicon (M1/M2/M3/M4) Mac.
- Python 3.9+ installed.

### Setup and Start
```bash
# 1. Create a virtual environment
python3 -m venv venv

# 2. Activate the environment
source venv/bin/activate

# 3. Install MLX LM
pip install --upgrade pip
pip install mlx-lm

# 4. Start the server (keep this terminal open)
# This will download the model if not already present
python3 -m mlx_lm.server --model mlx-community/Llama-3.2-1B-Instruct-4bit --port 8080
```

### Option B: Ollama (Universal)
If you prefer [Ollama](https://ollama.com), ensure it's running and pull your preferred model:
```bash
ollama pull llama3.2:1b
```
Then in the `docs_organiser` dashboard, add:
- **Model Name**: `llama3.2:1b`
- **API URL**: `http://localhost:11434/v1`

> [!TIP]
> You can mix and match providers! Add a local MLX model for speed and an Ollama model as a backup. Use the "Default" button in the dashboard to set your primary choice.

## 2. Installation

You can build the binary locally or install it to your `$GOPATH/bin`.

```bash
# Clone or download this repository
cd docs_organiser

# Build locally
make build

# OR Install to $GOPATH/bin
make install
```

Run the tool with source and destination directories. You can also customize processing limits via flags, environment variables, or a YAML configuration file.

### Quick Start
```bash
./docs_organiser -src "./messy" -dst "./clean"
```

### Using a Config File
The application automatically looks for a `config.yaml` in the current directory. You can also specify a custom path:
```bash
./docs_organiser -config ./my-config.yaml
```

### Configuration Options

You can configure the application using CLI flags, a YAML file, or environment variables. 

**Precedence Order:**
1. **CLI Flags** (Highest)
2. **YAML Config File**
3. **Environment Variables**
4. **Default Values** (Lowest)

| Flag | Environment Variable | YAML Key | Description | Default |
| :--- | :--- | :--- | :--- | :--- |
| `-src` | `DOCS_SRC` | `src` | Source directory (recursive) | **Required** |
| `-dst` | `DOCS_DST` | `dst` | Destination directory | **Required** |
| `-config`| - | - | Path to custom YAML config | `config.yaml` |
| `-ctx` | `DOCS_CTX` | `ctx` | Model context window size (tokens)| `4096` |
| `-limit` | `DOCS_LIMIT` | `limit` | Max extraction (chars) | `100000` |
| `-workers`| `DOCS_WORKERS`| `workers`| Processing workers | `5` |
| `-api` | `DOCS_API` | `api` | Default API URL | `http://localhost:8080/v1` |
| `-server_port`| `DOCS_SERVER_PORT`| `server_port`| App Server Dashboard Port | `8090` |
| `-metrics_port`| `DOCS_METRICS_PORT`| `metrics_port`| Prometheus Metrics Port | `8081` |

#### Example using Flags:
```bash
./docs_organiser -src "./messy" -dst "./clean" -ctx 8192
```

#### Example using Environment Variables:
```bash
export DOCS_LIMIT=200000
./docs_organiser -src "./messy" -dst "./clean"
```

> [!NOTE]
> Detailed logs have been removed for a cleaner production terminal experience. Progress is shown in real-time.

## GPU Acceleration

The `docs_organiser` leverages the MLX framework, which is designed to run efficiently on Apple Silicon GPUs. When you start the MLX server as described above, it will automatically utilize the GPU for model inference.

## Troubleshooting

- **Port Conflict**: If ports 8090 or 8081 are busy, use `make stop` to clear lingering processes.
- **Connection Refused**: Ensure your AI server (MLX or other) is running on the URL specified in the Model Pool.
- **Invalid JSON**: High-temperature sampling can sometimes cause models to output junk; the tool includes sanitizers, but upgrading to a better "Instruct" model is recommended.
- **Graceful Stop**: Use `Ctrl+C` or the dashboard Stop button to end the pipeline safely.
