This is a project that creates a `Distributed Content Filtering and Summarization System for Multiple Content Platforms`

Project core aims:

- use React for the frontend (display form for adding a URL of a website article to scrape, and displaying cached article results)
- use go as the core backend api (the frontend sends requests to it, if it is a data fetch it will use it's cached data and internal functions, if requires AI processing will forward request to a python server running a FastAPI app that runs an AI model)
- FastAPI python server that does web-scraping for the url it get as a request from the backend, processes summary and sentiment with ollama model, and POSTing back to the backend when result are finished

Additional goals:

- Redis:
  - A Message Queue or Pub/Sub
    use Redis as a lightweight message broker for communication between:
    Go backend (publisher/subscriber)
    FastAPI worker (processor and result-pusher)
    📌 Example pattern:
    Go publishes task to a Redis queue (e.g., process:queue)
    FastAPI listens to Redis queue, processes
    FastAPI publishes result to Redis Pub/Sub
    Go listens to result channel and stores result in Redis
    (If WebSocket is active) Go pushes it to frontend client
  - B Caching processed URLs
    Store results of processed URLs in Redis (e.g. with url_hash as key, value as JSON of result)
    Go backend checks Redis before forwarding request to AI
    If result exists: return immediately
    If not: forward to Python, and cache when response is done
- Updated data at the backend (retrived from the python ml server) needs to force an update on the frontend, so it means that there should be some state management + websocket callbacks from the backend that are recieved by the front (to any connected client)

Last Missions:

- Implement basic frontend with a form to add new article URLs and display cached results
- Add websockets communication and callbacks to backend and frotnend
- Implement a Task `status` and Task `id` to keep track of tasks in the backend (when the backend receives a new task it creates a new unique id for it with the status of "WORKING", when finished status changes to "SUCCESS" and caching results and send that result back to the frontend via WebSocket (However if task fails for whatever reason, the `status` is changed to "FAILED" and the websocket sends to the frontend an error message))
