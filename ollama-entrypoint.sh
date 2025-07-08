#!/bin/bash

# Custom entrypoint for Ollama container
echo "Starting Ollama with automatic model setup..."

# Start Ollama in the background
ollama serve &

# Wait for Ollama to be fully ready - NOT A MUST - it works without this loop as well(check API endpoint)
# echo "Waiting for Ollama to be ready..."
# while ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
#     echo "Ollama not ready yet, waiting..."
#     sleep 5
# done

# echo "Ollama is ready! Checking models..."

# Wait a bit more for GPU initialization to complete
sleep 10

# Pull the model if it doesn't exist
echo "Checking if model exists..."
if ! ollama list | grep -q "llama3.1:8b"; then
    echo "Pulling llama3.1:8b..."
    ollama pull llama3.1:8b
else
    echo "Model llama3.1:8b already exists"
fi

# Run the model (this keeps it loaded in memory)
echo "Running llama3.1:8b..."
ollama run llama3.1:8b &

# Keep the container running
wait