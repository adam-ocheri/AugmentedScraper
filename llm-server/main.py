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
from urllib.parse import urlparse
from bs4 import BeautifulSoup
from ollama import Client
import chromadb
import os
import re

EMBEDDING_MODEL_NAME = "nomic-embed-text:latest"

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


def chunk_text(text, max_chunk_size=1500):
    """Split text into chunks that fit within the token limit"""
    # Simple word-based chunking (rough approximation of tokens)
    words = text.split()
    chunks = []
    current_chunk = []
    current_size = 0

    for word in words:
        # Rough estimate: 1 word ≈ 1.3 tokens
        word_size = len(word.split()) * 1.3

        if current_size + word_size > max_chunk_size and current_chunk:
            chunks.append(" ".join(current_chunk))
            current_chunk = [word]
            current_size = word_size
        else:
            current_chunk.append(word)
            current_size += word_size

    if current_chunk:
        chunks.append(" ".join(current_chunk))

    return chunks


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
            # Chunk the text to fit within token limits
            logging.info(f"Embedding text for UUID: {uuid}")
            logging.info(f"Text length: {len(text)} characters")
            chunks = chunk_text(text, max_chunk_size=1500)
            logging.info(f"Split text into {len(chunks)} chunks for UUID: {uuid}")

            all_embeddings = []
            all_documents = []
            all_metadatas = []
            all_ids = []

            for i, chunk in enumerate(chunks):
                try:
                    logging.info(
                        f"Embedding chunk {i+1}/{len(chunks)} of UUID: {uuid} | Chunk length: {len(chunk)}"
                    )
                    response = self.client.embed(
                        model=EMBEDDING_MODEL_NAME, input=chunk
                    )
                    logging.info(
                        f"Generated embedding for chunk {i+1}, embedding length: {len(response['embeddings'])}"
                    )

                    # Fix the embedding format - flatten nested lists if needed
                    embeddings = response["embeddings"]
                    if isinstance(embeddings, list) and len(embeddings) > 0:
                        # If it's a nested list like [[[embeddings]]], flatten it
                        while isinstance(embeddings[0], list):
                            embeddings = embeddings[0]

                    logging.info(
                        f"Flattened embedding length for chunk {i+1}: {len(embeddings)}"
                    )

                    # Store chunk with metadata
                    chunk_id = f"{uuid}_chunk_{i}"
                    all_embeddings.extend(embeddings)
                    all_documents.append(chunk)
                    all_metadatas.append(
                        {
                            "uuid": str(uuid),
                            "chunk_index": i,
                            "total_chunks": len(chunks),
                        }
                    )
                    all_ids.append(chunk_id)

                    logging.info(
                        f"Generated embeddings for chunk {i+1}/{len(chunks)} of UUID: {uuid}"
                    )

                except Exception as e:
                    logging.error(f"Error embedding chunk {i} for UUID {uuid}: {e}")

            # Store all chunks in ChromaDB
            if all_embeddings:
                logging.info(
                    f"Storing {len(all_documents)} chunks in ChromaDB for UUID: {uuid}"
                )
                collection.add(
                    ids=all_ids,
                    embeddings=all_embeddings,
                    documents=all_documents,
                    metadatas=all_metadatas,
                )
                logging.info(
                    f"Successfully stored {len(all_documents)} chunks in ChromaDB for UUID: {uuid}"
                )

                # Verify the data was stored
                try:
                    count = collection.count(where={"uuid": str(uuid)})
                    logging.info(
                        f"Verified {count} chunks stored in ChromaDB for UUID: {uuid}"
                    )
                except Exception as e:
                    logging.error(
                        f"Error verifying ChromaDB storage for UUID {uuid}: {e}"
                    )
            else:
                logging.error(f"No embeddings generated for any chunks of UUID: {uuid}")

        except Exception as e:
            logging.error(f"Error in embed_text for UUID {uuid}: {e}")

    def retrieve_text(self, uuid, prompt):
        """Retrieve relevant context from ChromaDB based on the prompt"""
        try:
            logging.info(f"Retrieving context for UUID: {uuid} with prompt: {prompt}")

            response = self.client.embed(model=EMBEDDING_MODEL_NAME, input=prompt)
            logging.info(
                f"Generated embedding for prompt, embedding length: {len(response['embeddings'])}"
            )

            # Fix the embedding format - flatten nested lists if needed
            embeddings = response["embeddings"]
            if isinstance(embeddings, list) and len(embeddings) > 0:
                # If it's a nested list like [[[embeddings]]], flatten it
                while isinstance(embeddings[0], list):
                    embeddings = embeddings[0]

            logging.info(f"Flattened embeddings length: {len(embeddings)}")

            # query the collection for all chunks of the specific article
            results = collection.query(
                query_embeddings=[embeddings],
                n_results=3,  # Get top 3 most relevant chunks
                where={"uuid": str(uuid)},
            )

            logging.info(f"ChromaDB query results: {results}")

            if results["documents"] and len(results["documents"][0]) > 0:
                # Combine the most relevant chunks
                relevant_chunks = results["documents"][0]
                combined_context = " ".join(relevant_chunks)
                logging.info(
                    f"Retrieved {len(relevant_chunks)} chunks for UUID: {uuid}"
                )
                logging.info(f"Combined context: {combined_context[:200]}...")
                return combined_context
            else:
                logging.warning(f"No context found for UUID: {uuid}")
                logging.warning(f"ChromaDB results: {results}")
                return ""
        except Exception as e:
            logging.error(f"Error retrieving text from ChromaDB: {e}")
            logging.error(f"Exception details: {str(e)}")
            return ""

    def chat(self, text, uuid=None) -> str:
        """Chat with the model using context from the article"""
        logging.info(f"Generating chat response for model: {self.model_name}")

        # Retrieve relevant context from the article if UUID is provided
        context = ""
        if uuid:
            context = self.retrieve_text(uuid, text)
            if context:
                logging.info(f"Retrieved context for UUID: {uuid}")
            else:
                logging.warning(f"No context found for UUID: {uuid}")

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
            self.update_conversation_in_backend(uuid)

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

        self.embed_text(article_text, uuid)

        SUMMARY_INSTRUCTION = "You will be given a the text contents of a scraped webpage at the given url. You will then need to generate a summary of the webpage, and return the result as a Markdown string."
        SENTIMENT_INSTRUCTION = "You will be the summary of an article. You will then need to generate a sentiment of the webpage; return only the sentiment, no other text (e.g. 'positive', 'negative', 'neutral')"

        summary = self.generate_text(SUMMARY_INSTRUCTION, article_text)
        sentiment = self.generate_text(SENTIMENT_INSTRUCTION, summary)
        return {
            "summary": summary,
            "sentiment": sentiment,
        }


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
    # Start the worker thread
    thread = threading.Thread(target=redis_worker, daemon=True)
    thread.start()
    yield
    # (Optional cleanup code can go here)


app = FastAPI(lifespan=lifespan)


def is_valid_url(url: str) -> bool:
    """Check if the given string is a valid URL"""
    try:
        result = urlparse(url)
        return all([result.scheme, result.netloc])
    except:
        return False


def scrape_url(url: str) -> str:
    """Scrape the text contents of the webpage at the given url"""
    if not is_valid_url(url):
        raise ValueError("Invalid URL")

    response = requests.get(url)
    if response.status_code != 200:
        raise ValueError("Failed to fetch the URL")

    soup = BeautifulSoup(response.text, "html.parser")
    text = soup.get_text()
    logging.info(f"Scraped text: {text}")

    return text


# TODO: Refactor this to run on a background thread
def process_article(url: str, url_task_data: str, url_task_key: str, task_uuid: str):
    text = scrape_url(url)
    result = LLM.process_article(text, task_uuid)

    # Generate result (this is where you'd integrate with your actual LLM)
    result = {
        "url": url,
        "summary": result["summary"],
        "sentiment": result["sentiment"],
        "conversation": [],  # Initialize empty conversation array
        # "result": result,
        # "key_points": [
        #     f"Key point 1 about {url}",
        #     f"Key point 2 about {url}",
        #     f"Key point 3 about {url}",
        # ],
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
                    # "result": result,
                    # "key_points": [
                    #     f"Key point 1 about {url}",
                    #     f"Key point 2 about {url}",
                    #     f"Key point 3 about {url}",
                    # ],
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


@app.get("/stats")
def get_stats():
    """Get current statistics about tasks and queue"""
    try:
        queue_length = r.llen("queue:tasks")

        # Count tasks by status
        pending_count = 0
        processing_count = 0
        done_count = 0
        failed_count = 0

        # Get all status keys
        status_keys = r.keys("status:*")
        for key in status_keys:
            status = r.get(key)
            if status == "pending":
                pending_count += 1
            elif status == "processing":
                processing_count += 1
            elif status == "done":
                done_count += 1
            elif status == "failed":
                failed_count += 1

        return {
            "queue_length": queue_length,
            "tasks_pending": pending_count,
            "tasks_processing": processing_count,
            "tasks_done": done_count,
            "tasks_failed": failed_count,
        }
    except Exception as e:
        logging.error(f"Error getting stats: {e}")
        return {"error": str(e)}


@app.get("/debug/chromadb")
def debug_chromadb():
    """Debug endpoint to check ChromaDB status"""
    try:
        # Get collection info
        collection_info = {
            "name": collection.name,
            "count": collection.count(),
            "metadata": collection.metadata,
        }

        # Test embedding generation
        test_text = "This is a test embedding"
        test_response = LLM.client.embed(model=EMBEDDING_MODEL_NAME, input=test_text)

        return {
            "collection_info": collection_info,
            "embedding_test": {
                "model": EMBEDDING_MODEL_NAME,
                "test_text": test_text,
                "embedding_length": len(test_response["embeddings"]),
                "success": True,
            },
        }
    except Exception as e:
        logging.error(f"Error in ChromaDB debug: {e}")
        return {"error": str(e)}


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
