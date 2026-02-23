# MLX-Powered File Mover

A standalone Go application that uses a local MLX AI model (via HTTP) to intelligently categorize and move documents.

## Features
- **GPU-Accelerated**: Leverages Apple Silicon GPU by default via MLX for fast categorization.
- **Intelligent Categorization**: Uses Llama-3 models to analyze and organize your messy documents.
- **Production Ready**: Clean terminal output with progress tracking, concurrency control, and graceful shutdown.
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

> [!TIP]
> It is recommended to run the server in a separate terminal window and keep it active while using the `docs_organiser`.

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
| `-api` | `DOCS_API` | `api` | MLX server URL | `localhost:8080` |
| `-model` | `DOCS_MODEL` | `model` | AI Model name | `Llama-3.2-1B` |

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

- **"connection refused"**: Ensure your MLX server is running on the correct port (default 8080).
- **"invalid character '<' in JSON"**: The model might be outputting non-JSON content. The tool includes cleaners to handle this, but if it persists, ensure you are using a decent "Instruct" model.
- **Graceful Stop**: You can stop the process at any time using `Ctrl+C`. The pipeline will stop safely.
