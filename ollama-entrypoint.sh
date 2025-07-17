#!/bin/bash

# Custom entrypoint for Ollama container
echo "Starting Ollama with automatic model setup..."

# Start Ollama in the background
ollama serve &

# Wait a bit more for GPU initialization to complete
# sleep 10

# Pull the model if it doesn't exist
echo "Checking if model exists..."
if ! ollama list | grep -q "llama3.1:8b"; then
    echo "Pulling llama3.1:8b..."
    ollama pull llama3.1:8b
else
    echo "Model llama3.1:8b already exists"
fi

if ! ollama list | grep -q "nomic-embed-text:latest"; then
    echo "Pulling nomic-embed-text:latest..."
    ollama pull nomic-embed-text:latest
else
    echo "Model nomic-embed-text:latest already exists"
fi

# Run the models (this keeps them loaded in memory)
echo "Running nomic-embed-text:latest..."
ollama run nomic-embed-text:latest &
echo "Running llama3.1:8b..."
ollama run llama3.1:8b &

# Keep the container running
wait