# Project missions

# MISSION 1 | Status: Completed | Details: Need to set up the basic `backend` and `llm-server` services

    - A) Backend needs to accept new requests on the `/submit` route, first check if the requested url has already been processed and cached
        + If the requested URL has already been processed and cached, return the cached results and finish
        + If not cached, add new task to the queue (with Status and UUID) to be processed at the LLM Server
    - B) LLM Server needs to listen to incoming tasks and pull them from the queue, process and publish the results back
    - C) Once a task has finished, it's status needs to be updated, and it's results need to be cached. When the LLM server completes a task, it should:
        + Update cache:<url> with the result
        + Update url_task:<url> with final status
        + Update status:<uuid> with final status

# MISSION 2 | Status: Completed | Details: Create a function in the llm-server that scrapes the webpage at the given url, and returns the text contents of the page article

# MISSION 3 | Status: Completed | Details: Add Ollama integration to actually get generative results

# MISSION 4 | Status: Completed | Details: Add websocket to the Go backend, and add a push notifications to realtime update channel once `process:results` is triggered

# MISSION 5 | Status: Completed | Details: Create frontend ui to support the system

    - A) Created a modern React frontend with Next.js and Tailwind CSS
    - B) Added Dockerfile for production deployment
    - C) Added Dockerfile.dev for development with hot reloading
    - D) Integrated with docker-compose.yml for complete system orchestration
    - E) Implemented real-time WebSocket updates for task status
    - F) Added task history and result display
    - G) Created responsive UI with proper error handling

# MISSION 6 | Status: Completed | Details: What makes the `cache:` key actually behave like a cache? I think the caching mechanism is still a bit off

    - A) Set cached results to EXPIRE after a set time
    - B) Add a Postgres DB to store article results (model: {"url": str, "summary": str, "sentiment" str, "conversation": Array({"role": str, "content" str})})
    - C) Update the backed `/submit` route;
        + if not cached, first look if data exists in db, and if so retreive and cache it in Redis
        + if not cached and does not exist in DB, start new task
    - D) !!! Might need to add `"conversation": Array({"role": str, "content" str})` to the cache as well

# MISSION 7 | Status: Pending | Details: Add early validation to identify that the incoming string is indeed a valid URL string (at the go backend `/submit` route), and if not valid return an error message to the frontend saying "Invalid URL used!"

    + A) Also, will need to add validation and string manipulations to ensure we are redundantly not re-processing URLs; i.e if already have `https://www.example.com` stored, if I then add `http://www.example.com` or `www.example.com` or `example.com`, it should all be treated as the same url (because it is the SAME url)

# MISSION 8 | Status: Pending | Details: Re-consider possible redundancy at the `status:` redis Key Pattern; since there is already the `url_task:` that has a status as well, and what actual use do you have for the `status/{uuid}` route anyway?

# MISSION 9 | Status: Pending | Details: Add some check in the llm-server to know when the ollama model has finished downloading and is ready for use. Then it should POST to a route in the go backend `/inform-model-loaded`, and add a websocket callback to update the frontend, informing it that it is now allowed to make requests and start (it should not allow users use the system while the model is still downloading - it should display a nice loading animation instead)

# MISSION 10 | Status: Completed | Details: Frontend needs to display all articles/tasks on a side panel as a list of buttons

    - A) Instead of a `result` state, there should be `activeArticle` state
    - B) When a button is clicked, the UI updates the currently displayed `activeArticle` to the corresponding article/task that belongs to the button
    - C) Then there might be a need for a `useEffect` that is triggered when the `activeArticle` has been changed, to fetch that article/task from the backend
    - D) NOTE: since users may make concurrent requests, need to make sure that the `activeArticle` is only updated via the websocket (instead of the old `result` state) only if the `activeArticle.uuid === payload.uuid`

# MISSION 11 | Status: Pending | Details: add full support for having a conversation after getting result from article

    - A) Add support for the `ConversationEntry` so that conversation data can be stored in the DB and Redis cache
    - B) Once added conversations array to the DB/Cache data structures, ensure that nothing breaks in the current implementation
    - C) Add ChromaDB and embeddings logic to make each prompt have the additional context it needs for a better answer
        - Add in the ollama-entrypoint.sh commands loading an embedding model, and refactor the ModelInterface class to support internally the embedding model
        - Add chromadb to the requirements.txt to have the package installed, and add a container in the docker-compose.yml setup with the chromadb image
        - Once a new article is processed, the article text needs to be embedded and stored in chromadb (but also think how would you relate it to to the DB/Cache data, probably every article stored in chromadb needs to have the same uuid as the Task uuid)
        - On `ModelInterface.chat` need to query the vector db and add aditional context to the user's prompt
        - Before `ModelInterface.chat` is finished, it needs to update the conversation in the DB and Cache
    - D) Add frontend support for the conversation
        - Update the tasks state var data structure to support a conversation array
        - Add another div at the bottom of the `activeArticle` that has an input element (for writing the next prompt), a submit button, and above it the entire history of the conversation for this article
        - Need to make sure that when the `activeArticle` changes, the ModelInterface.memory (in the llm-server) is being re-assigned with the newly updated current active conversation
        - Everytime the llm-server adds a new item to the conversation, the conversation needs to be updated in the DB and the Cache
    - E) OPTIONAL - Try adding another LLM step that attempts to clean the article text from unrelated text, since we are using a dirty web scrape
        + This means to simply add another prompt process with a prompt like `model.generate_text(text, "I am sending you a text of a scraped web page article. Please identify if there are unnecessary and unrelated characters, words or sentences that does")`

# MISSION 12 | Status: Pending | Details: Potential issue with concurrent requests - the python subscriber worker can potentially become stuck when trying to process multiple requests number exceeding the "OLLAMA_MAX_THREADS" or whatever that env var's name is. So there should probably be a refactor moving that bit of code to run on a background thread, clearing the pubsub loop to be able to process newly incoming events without a huckle.

    - A) Ollama has an internal queue keeping requests exceeding num "OLLAMA_MAX_THREADS" in it, but need to ensure there isn't a way for it to break (research if ollama has a cache folder you can add to a volume, if it includes the internal queue data)

# MISSION 13 | Status: Pending | Details: For a better UX, can make the frontend notify the user about the stages of the processing of his request (for example, "Reading URL contents...", "Cleaning article from unrelated text...", "generating summary...", "generating sentiment...")

    - A) This means creating websocket/pubsub callbacks for each of these steps
    - B) These callbacks would need to be triggered by the llm-server, pushing messages to the backend server
    - C) The backend server then forwards these messages to the front

# MISSION 14 | Status: Pending | Details: Frontend should also be able to return cache hit! Right now it waits only for the websocket to return the data || or maybe it should return a message saying 'already exists' to not over-complicate things
