# Project missions

# MISSION 1 | Status: Pending | Details: Need to set up the basic `backend` and `llm-server` services

    - A) Backend needs to accept new requests on the `/submit` route, first check if the requested url has already been processed and cached
        + If the requested URL has already been processed and cached, return the cached results and finish
        + If not cached, add new task to the queue (with Status and UUID) to be processed at the LLM Server
    - B) LLM Server needs to listen to incoming tasks and pull them from the queue, process and publish the results back
    - C) Once a task has finished, it's status needs to be updated, and it's results need to be cached. When the LLM server completes a task, it should:
        + Update cache:<url> with the result
        + Update url_task:<url> with final status
        + Update status:<uuid> with final status

# MISSION 2 | Status: Pending | Details: Create a function in the llm-server that scrapes the webpage at the given url, and returns the text contents of the page article

# MISSION 3 | Status: Pending | Details: Add Ollama integration to actually get generative results

# MISSION 4 | Status: Pending | Details: Add websocket to the Go backend, and add a push notifications to realtime update channel once `process:results` is triggered

# MISSION 5 | Status: Pending | Details: Create frontend ui to support the system

# MISSION 6 | Status: Pending | Details: Add early validation to identify that the incoming string is indeed a valid URL string (at the `/submit` route)
