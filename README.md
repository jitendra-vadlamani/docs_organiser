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

## 3. Usage

Run the tool with the source and destination directories. Wrap paths in quotes if they contain spaces.

```bash
./docs_organiser \
  -src "/Path/To/Your/Messy Docs" \
  -dst "/Path/To/Organized Docs" \
  -workers 5
```

### Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-src` | Source directory to scan (recursive) | **Required** |
| `-dst` | Destination directory to move files | **Required** |
| `-workers` | Number of concurrent workers | `5` |
| `-api` | URL of the MLX server | `http://localhost:8080/v1` |
| `-model` | Model name sent in API request | `mlx-community/Llama-3.2-1B-Instruct-4bit` |

> [!NOTE]
> Detailed logs have been removed for a cleaner production terminal experience. Progress is shown in real-time.

## GPU Acceleration

The `docs_organiser` leverages the MLX framework, which is designed to run efficiently on Apple Silicon GPUs. When you start the MLX server as described above, it will automatically utilize the GPU for model inference.

## Troubleshooting

- **"connection refused"**: Ensure your MLX server is running on the correct port (default 8080).
- **"invalid character '<' in JSON"**: The model might be outputting non-JSON content. The tool includes cleaners to handle this, but if it persists, ensure you are using a decent "Instruct" model.
- **Graceful Stop**: You can stop the process at any time using `Ctrl+C`. The pipeline will stop safely.
