import json
import time
import redis
import logging
import sys
import threading
from fastapi import FastAPI
import uvicorn
from contextlib import asynccontextmanager
import requests
from ollama import Client
import chromadb
import os
from utils import chunk_text, scrape_url

EMBEDDING_MODEL_NAME = "nomic-embed-text:latest"
LLM_MODEL_NAME = "llama3.1:8b"

# Initialize ChromaDB client with proper configuration
chromadb_host = os.getenv("CHROMADB_HOST", "http://localhost:8000")
chroma_db_client = chromadb.HttpClient(host=chromadb_host)

# Create or get the collection
try:
    collection = chroma_db_client.create_collection(name="articles")
    logging.info("Created new ChromaDB collection: articles")
except Exception as e:
    # Collection might already exist
    collection = chroma_db_client.get_collection(name="articles")
    logging.info("Using existing ChromaDB collection: articles")

# Global flag for model readiness
models_ready = False
models_ready_lock = threading.Lock()


def check_models_ready():
    """Check if required models are loaded in Ollama"""
    try:
        logging.error(
            "------------------------------>Checking models ready<------------------------------"
        )
        response = requests.get("http://ollama:11434/api/ps", timeout=5)
        if response.status_code == 200:
            logging.error(f"Ollama response: {response.json()}")
            data = response.json()
            loaded_models = [model["name"] for model in data.get("models", [])]

            required_models = [LLM_MODEL_NAME]  # , EMBEDDING_MODEL_NAME]
            ready = all(model in loaded_models for model in required_models)

            logging.error(f"Loaded models: {loaded_models}")
            logging.error(f"Required models: {required_models}")
            logging.error(f"Models ready: {ready}")

            return ready
    except Exception as e:
        logging.error(f"Error checking models: {e}")
    return False


def notify_backend_models_ready():
    """Notify the backend that models are ready"""
    try:
        backend_url = os.getenv("BACKEND_URL", "http://backend:8080")
        response = requests.post(f"{backend_url}/inform-model-loaded", timeout=10)
        if response.status_code == 200:
            logging.info("Successfully notified backend that models are ready")
        else:
            logging.error(f"Failed to notify backend: {response.status_code}")
    except Exception as e:
        logging.error(f"Error notifying backend: {e}")


def model_readiness_checker():
    """Background thread to check model readiness"""
    global models_ready

    # Check if models are already ready (in case of container restart)
    if check_models_ready():
        with models_ready_lock:
            models_ready = True
        logging.info("Models already loaded and ready!")
        notify_backend_models_ready()
        return

    # Polling intervals: start fast, then slow down
    # polling_intervals = [2, 2, 2, 5, 5, 10, 10, 10]
    # current_interval_index = 0

    logging.info("Starting model readiness checker...")

    while not models_ready:
        if check_models_ready():
            with models_ready_lock:
                models_ready = True

            logging.info("All required models are loaded and ready!")
            notify_backend_models_ready()
            break

        # polling interval
        interval = 5

        logging.info(f"Models not ready yet, checking again in {interval} seconds...")
        time.sleep(interval)

    logging.info("Model readiness checker finished")


class ModelInterface:
    def __init__(self, model_name: str):
        logging.info(f"Initializing ModelInterface with model: {model_name}")

        self.client = Client(host="http://ollama:11434")
        self.model_name = model_name

        self.memory = [
            {
                "role": "system",
                "content": "You are a helpful assistant that can answer questions and help with tasks.",
            }
        ]

    def get_model(self) -> str:
        return self.client.models.get(self.model_name)

    def generate_text(self, instruction, text) -> str:
        logging.info(f"Generating text for model: {self.model_name}")
        response = self.client.chat(
            model=self.model_name,
            messages=[
                {
                    "role": "system",
                    "content": instruction,
                },
                {
                    "role": "user",
                    "content": text,
                },
            ],
        )
        logging.info(f"LLM response: {response['message']['content']}")
        return response["message"]["content"]

    def embed_text(self, text, uuid):
        """Embed article text and store in ChromaDB with UUID metadata"""
        try:
            # Validate input
            if not text or not text.strip():
                logging.error(f"Empty or invalid text provided for UUID: {uuid}")
                return

            if not uuid:
                logging.error("No UUID provided for embedding")
                return

            # Chunk the text to fit within token limits
            logging.info(f"Embedding text for UUID: {uuid}")
            logging.info(f"Text length: {len(text)} characters")
            chunks = chunk_text(text, max_chunk_size=1500)
            logging.info(f"Split text into {len(chunks)} chunks for UUID: {uuid}")

            if not chunks:
                logging.error(f"No chunks generated for UUID: {uuid}")
                return

            all_embeddings = []
            all_documents = []
            all_metadatas = []
            all_ids = []

            successful_chunks = 0
            for i, chunk in enumerate(chunks):
                try:
                    if not chunk or not chunk.strip():
                        logging.warning(f"Skipping empty chunk {i} for UUID: {uuid}")
                        continue

                    logging.info(
                        f"Embedding chunk {i+1}/{len(chunks)} of UUID: {uuid} | Chunk length: {len(chunk)}"
                    )

                    # Validate chunk before embedding
                    if len(chunk) < 10:  # Skip very short chunks
                        logging.warning(
                            f"Skipping very short chunk {i} for UUID: {uuid}"
                        )
                        continue

                    response = self.client.embed(
                        model=EMBEDDING_MODEL_NAME, input=chunk
                    )

                    if not response or "embeddings" not in response:
                        logging.error(
                            f"Invalid embedding response for chunk {i} of UUID: {uuid}"
                        )
                        continue

                    logging.info(
                        f"Generated embedding for chunk {i+1}, embedding length: {len(response['embeddings'])}"
                    )

                    # Fix the embedding format - flatten nested lists if needed
                    embeddings = response["embeddings"]
                    if isinstance(embeddings, list) and len(embeddings) > 0:
                        # If it's a nested list like [[[embeddings]]], flatten it
                        while isinstance(embeddings[0], list):
                            embeddings = embeddings[0]

                    # Validate embedding
                    if not embeddings or len(embeddings) == 0:
                        logging.error(
                            f"Empty embedding generated for chunk {i} of UUID: {uuid}"
                        )
                        continue

                    logging.info(
                        f"Flattened embedding length for chunk {i+1}: {len(embeddings)}"
                    )

                    # Store chunk with metadata
                    chunk_id = f"{uuid}_chunk_{i}"
                    # FIXED: Add each embedding as a separate list item, not extend
                    all_embeddings.append(embeddings)
                    all_documents.append(chunk)
                    all_metadatas.append(
                        {
                            "uuid": str(uuid),
                            "chunk_index": i,
                            "total_chunks": len(chunks),
                        }
                    )
                    all_ids.append(chunk_id)
                    successful_chunks += 1

                    logging.info(
                        f"Generated embeddings for chunk {i+1}/{len(chunks)} of UUID: {uuid}"
                    )

                except Exception as e:
                    logging.error(f"Error embedding chunk {i} for UUID {uuid}: {e}")

            # Validate that we have matching lengths before storing
            if (
                len(all_embeddings) != len(all_documents)
                or len(all_documents) != len(all_metadatas)
                or len(all_metadatas) != len(all_ids)
            ):
                logging.error(
                    f"Length mismatch in ChromaDB data for UUID {uuid}: embeddings={len(all_embeddings)}, documents={len(all_documents)}, metadatas={len(all_metadatas)}, ids={len(all_ids)}"
                )
                return

            # Store all chunks in ChromaDB
            if all_embeddings and successful_chunks > 0:
                logging.info(
                    f"Storing {len(all_documents)} chunks in ChromaDB for UUID: {uuid}"
                )

                try:
                    collection.add(
                        ids=all_ids,
                        embeddings=all_embeddings,
                        documents=all_documents,
                        metadatas=all_metadatas,
                    )
                    logging.info(
                        f"Successfully stored {len(all_documents)} chunks in ChromaDB for UUID: {uuid}"
                    )
                except Exception as e:
                    logging.error(f"Error adding to ChromaDB for UUID {uuid}: {e}")
                    return

                # Verify the data was stored - FIXED: Remove 'where' parameter for compatibility
                try:
                    # Get all documents and filter by UUID in Python instead
                    all_results = collection.get()
                    if all_results and "metadatas" in all_results:
                        uuid_count = sum(
                            1
                            for metadata in all_results["metadatas"]
                            if metadata and metadata.get("uuid") == str(uuid)
                        )
                        logging.info(
                            f"Verified {uuid_count} chunks stored in ChromaDB for UUID: {uuid}"
                        )
                    else:
                        logging.info(
                            f"Verified chunks stored in ChromaDB for UUID: {uuid}"
                        )
                except Exception as e:
                    logging.error(
                        f"Error verifying ChromaDB storage for UUID {uuid}: {e}"
                    )
            else:
                logging.error(f"No embeddings generated for any chunks of UUID: {uuid}")

        except Exception as e:
            logging.error(f"Error in embed_text for UUID {uuid}: {e}")
            import traceback

            logging.error(f"Traceback: {traceback.format_exc()}")

    def retrieve_text(self, uuid, prompt):
        """Retrieve relevant context from ChromaDB based on the prompt"""
        try:
            # Validate input
            if not uuid:
                logging.error("No UUID provided for text retrieval")
                return ""

            if not prompt or not prompt.strip():
                logging.error("Empty or invalid prompt provided for text retrieval")
                return ""

            logging.info(f"Retrieving context for UUID: {uuid} with prompt: {prompt}")

            # Generate embedding for the prompt
            try:
                response = self.client.embed(model=EMBEDDING_MODEL_NAME, input=prompt)
                logging.info(
                    f"Generated embedding for prompt, embedding length: {len(response['embeddings'])}"
                )
            except Exception as e:
                logging.error(f"Error generating embedding for prompt: {e}")
                return ""

            # Fix the embedding format - flatten nested lists if needed
            embeddings = response["embeddings"]
            if isinstance(embeddings, list) and len(embeddings) > 0:
                # If it's a nested list like [[[embeddings]]], flatten it
                while isinstance(embeddings[0], list):
                    embeddings = embeddings[0]

            logging.info(f"Flattened embeddings length: {len(embeddings)}")

            # FIXED: Query without 'where' parameter for compatibility, then filter results
            try:
                results = collection.query(
                    query_embeddings=[embeddings],
                    n_results=10,  # Get more results to filter from
                )
            except Exception as e:
                logging.error(f"Error querying ChromaDB: {e}")
                return ""

            logging.info(f"ChromaDB query results: {results}")

            # Filter results by UUID in Python
            if results["documents"] and len(results["documents"][0]) > 0:
                filtered_documents = []
                filtered_metadatas = results.get("metadatas", [[]])[0]

                for i, metadata in enumerate(filtered_metadatas):
                    if metadata and metadata.get("uuid") == str(uuid):
                        if i < len(results["documents"][0]):
                            filtered_documents.append(results["documents"][0][i])

                # Take top 3 most relevant chunks for this UUID
                relevant_chunks = filtered_documents[:3]

                if relevant_chunks:
                    combined_context = " ".join(relevant_chunks)
                    logging.info(
                        f"Retrieved {len(relevant_chunks)} chunks for UUID: {uuid}"
                    )
                    logging.info(f"Combined context: {combined_context[:200]}...")
                    return combined_context
                else:
                    logging.warning(
                        f"No context found for UUID: {uuid} after filtering"
                    )
                    return ""
            else:
                logging.warning(f"No context found for UUID: {uuid}")
                logging.warning(f"ChromaDB results: {results}")
                return ""
        except Exception as e:
            logging.error(f"Error retrieving text from ChromaDB: {e}")
            logging.error(f"Exception details: {str(e)}")
            import traceback

            logging.error(f"Traceback: {traceback.format_exc()}")
            return ""

    def chat(self, text, uuid=None) -> str:
        """Chat with the model using context from the article"""
        logging.info(f"Generating chat response for model: {self.model_name}")

        # Retrieve relevant context from the article if UUID is provided
        context = ""
        if uuid:
            try:
                context = self.retrieve_text(uuid, text)
                if context:
                    logging.info(f"Retrieved context for UUID: {uuid}")
                else:
                    logging.warning(f"No context found for UUID: {uuid}")
            except Exception as e:
                logging.warning(
                    f"Failed to retrieve context for UUID {uuid}, continuing without context: {e}"
                )
                context = ""

        # Prepare the prompt with context
        if context:
            enhanced_prompt = f"Use this context from the article: {context}\n\n to answer the following question: {text}"
        else:
            enhanced_prompt = text

        self.memory.append(
            {
                "role": "user",
                "content": enhanced_prompt,
            }
        )

        response = self.client.chat(model=self.model_name, messages=self.memory)
        generated_answer = response["message"]["content"]

        self.memory.append(
            {
                "role": "assistant",
                "content": generated_answer,
            }
        )

        logging.info(f"LLM chat response: {generated_answer}")

        # Update the conversation array in the Postgres DB and cache
        if uuid:
            try:
                self.update_conversation_in_backend(uuid)
            except Exception as e:
                logging.warning(f"Failed to update conversation for UUID {uuid}: {e}")

        return generated_answer

    def update_conversation_in_backend(self, uuid: str):
        """Update the conversation in the backend database and cache"""
        try:
            # Convert memory to the format expected by the backend
            conversation_entries = []
            for message in self.memory:
                if message["role"] != "system":  # Skip system messages
                    conversation_entries.append(
                        {"role": message["role"], "content": message["content"]}
                    )

            # Prepare the request payload
            payload = {"Uuid": uuid, "Conversation": conversation_entries}

            # Send request to backend
            backend_url = "http://backend:8080/conversation/update"
            response = requests.post(
                backend_url,
                json=payload,
                headers={"Content-Type": "application/json"},
                timeout=10,
            )

            if response.status_code == 200:
                logging.info(f"Successfully updated conversation for UUID: {uuid}")
            else:
                logging.error(
                    f"Failed to update conversation for UUID: {uuid}. Status: {response.status_code}, Response: {response.text}"
                )

        except Exception as e:
            logging.error(f"Error updating conversation for UUID {uuid}: {e}")

    def load_conversation_from_backend(self, uuid: str) -> bool:
        """Load existing conversation from backend for the given UUID"""
        try:
            # Try to get existing conversation from backend database directly
            backend_url = f"http://backend:8080/tasks"
            response = requests.get(backend_url, timeout=5)

            if response.status_code == 200:
                tasks_data = response.json()
                # Find the task with matching UUID
                for task in tasks_data.get("tasks", []):
                    if task.get("uuid") == uuid:
                        # Load existing conversation if available
                        if "conversation" in task and task["conversation"]:
                            self.reset_memory()  # Clear current memory
                            # Add existing conversation to memory
                            for entry in task["conversation"]:
                                self.memory.append(
                                    {
                                        "role": entry.get("role", "user"),
                                        "content": entry.get("content", ""),
                                    }
                                )
                            logging.info(
                                f"Loaded existing conversation for UUID: {uuid} with {len(task['conversation'])} entries"
                            )
                            return True
                        else:
                            logging.info(f"No conversation found for UUID: {uuid}")
                        break
            return False
        except Exception as e:
            logging.warning(
                f"Could not load existing conversation for UUID {uuid}: {e}"
            )
            return False

    def reset_memory(self):
        """Reset the conversation memory for a new article"""
        self.memory = [
            {
                "role": "system",
                "content": "You are a helpful assistant that can answer questions and help with tasks.",
            }
        ]
        logging.info("Reset conversation memory")

    def process_article(self, article_text, uuid):
        """Process article text and generate embeddings, summary, and sentiment"""
        try:
            # Try to embed the text, but don't fail if it doesn't work
            try:
                self.embed_text(article_text, uuid)
                logging.info(f"Successfully embedded article for UUID: {uuid}")
            except Exception as e:
                logging.warning(
                    f"Failed to embed article for UUID {uuid}, but continuing with processing: {e}"
                )
                # Continue processing even if embedding fails

            SUMMARY_INSTRUCTION = "You will be given a the text contents of a scraped webpage at the given url. You will then need to generate a summary of the webpage, and return the result as a Markdown string."
            SENTIMENT_INSTRUCTION = "You will be the summary of an article. You will then need to generate a sentiment of the webpage; return only the sentiment, no other text (e.g. 'positive', 'negative', 'neutral')"

            summary = self.generate_text(SUMMARY_INSTRUCTION, article_text)
            sentiment = self.generate_text(SENTIMENT_INSTRUCTION, summary)

            return {
                "summary": summary,
                "sentiment": sentiment,
            }
        except Exception as e:
            logging.error(f"Error in process_article for UUID {uuid}: {e}")
            # Return a default result to prevent complete failure
            return {
                "summary": "Error processing article",
                "sentiment": "neutral",
            }


# LLM interface singleton
LLM = ModelInterface("llama3.1:8b")

# Configure logging to output to stdout
logging.basicConfig(
    stream=sys.stdout,
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)

logging.info("Starting LLM server: ")

# Shared Redis connection
r = redis.Redis(host="redis", port=6379, db=0, decode_responses=True)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Start the model readiness checker thread
    model_checker_thread = threading.Thread(target=model_readiness_checker, daemon=True)
    model_checker_thread.start()

    # Start the worker thread
    worker_thread = threading.Thread(target=redis_worker, daemon=True)
    worker_thread.start()

    yield
    # (Optional cleanup code can go here)


app = FastAPI(lifespan=lifespan)


def redis_worker():
    logging.info("LLM worker started. Listening for tasks on queue:tasks...")
    while True:
        try:
            # Wait for tasks from the new queue
            task = r.blpop("queue:tasks", timeout=1)
            if task:
                _, raw_data = task
                data = json.loads(raw_data)
                url = data["url"]
                task_uuid = data["uuid"]

                logging.info(f"Processing task {task_uuid} for URL: {url}")

                # Update task status to "processing"
                r.set(f"status:{task_uuid}", "processing")

                # Update URL task mapping status
                url_task_key = f"url_task:{url}"
                url_task_data = r.get(url_task_key)
                if url_task_data:
                    url_task = json.loads(url_task_data)
                    url_task["status"] = "processing"
                    r.set(url_task_key, json.dumps(url_task))

                # Generative step ----------------
                text = scrape_url(url)
                result = LLM.process_article(text, task_uuid)

                # Generate result (this is where you'd integrate with your actual LLM)
                result = {
                    "url": url,
                    "summary": result["summary"],
                    "sentiment": result["sentiment"],
                    "conversation": [],  # Initialize empty conversation array
                    "processed_at": time.time(),
                }

                # Convert result to JSON string for caching
                result_json = json.dumps(result)

                # MISSION 1.C: Cache the result
                cache_key = f"cache:{url}"
                r.set(cache_key, result_json)
                logging.info(f"Cached result for URL: {url}")

                # Update task status to "done"
                r.set(f"status:{task_uuid}", "done")

                # Update URL task mapping to "done"
                if url_task_data:
                    url_task = json.loads(url_task_data)
                    url_task["status"] = "done"
                    r.set(url_task_key, json.dumps(url_task))

                # Publish result to results channel (for real-time updates if needed)
                r.publish(
                    "process:results",
                    json.dumps({"uuid": task_uuid, "url": url, "result": result}),
                )

                logging.info(f"Completed task {task_uuid} for URL: {url}")

        except Exception as e:
            logging.error(f"Error processing task: {e}")
            # If there was an error, update task status to "failed"
            if "task_uuid" in locals():
                r.set(f"status:{task_uuid}", "failed")
                # Update URL task mapping to "failed"
                if "url" in locals():
                    url_task_key = f"url_task:{url}"
                    url_task_data = r.get(url_task_key)
                    if url_task_data:
                        url_task = json.loads(url_task_data)
                        url_task["status"] = "failed"
                        r.set(url_task_key, json.dumps(url_task))
            time.sleep(1)  # Brief pause before retrying


@app.get("/health")
def health_check():
    return {"status": "ok", "service": "llm-server"}


@app.get("/is-model-loaded")
def is_model_loaded():
    """Check if required models are loaded and ready"""
    global models_ready

    with models_ready_lock:
        current_status = models_ready

    # Also check Redis for the flag (in case of container restart)
    try:
        redis_flag = r.get("models:ready")
        redis_ready = redis_flag == "true"
    except Exception as e:
        logging.error(f"Error checking Redis flag: {e}")
        redis_ready = False

    # Models are ready if either the global flag is True or Redis flag is set
    ready = current_status or redis_ready

    return {"ready": ready, "models_ready": current_status, "redis_ready": redis_ready}


@app.post("/chat")
def chat_endpoint(request: dict):
    """Handle chat conversations with article context"""
    try:
        uuid = request.get("uuid")
        message = request.get("message")

        if not uuid or not message:
            return {"error": "Missing uuid or message"}

        # Load existing conversation from backend if available
        if not LLM.load_conversation_from_backend(uuid):
            # If we can't load existing conversation, start fresh
            LLM.reset_memory()

        # Generate response with context
        response = LLM.chat(message, uuid)

        return {"uuid": uuid, "response": response, "success": True}
    except Exception as e:
        logging.error(f"Error in chat endpoint: {e}")
        return {"error": str(e), "success": False}


if __name__ == "__main__":
    logging.info("Starting FastAPI app server")
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=False)
