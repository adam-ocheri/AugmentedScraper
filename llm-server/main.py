import json
import time
import redis
import logging
import sys
import threading
from fastapi import FastAPI
import uvicorn
from contextlib import asynccontextmanager

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

                # Simulate AI processing delay
                time.sleep(3)

                # Generate result (this is where you'd integrate with your actual LLM)
                result = {
                    "url": url,
                    "summary": f"AI-generated summary of {url}",
                    "sentiment": "positive",
                    "key_points": [
                        f"Key point 1 about {url}",
                        f"Key point 2 about {url}",
                        f"Key point 3 about {url}",
                    ],
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


if __name__ == "__main__":
    logging.info("Starting FastAPI app server")
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=False)
