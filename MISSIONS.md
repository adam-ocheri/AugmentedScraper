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

# MISSION 4 | Status: Pending | Details: Add websocket to the Go backend, and add a push notifications to realtime update channel once `process:results` is triggered

# MISSION 5 | Status: Pending | Details: Create frontend ui to support the system

# MISSION 6 | Status: Pending | Details: Add early validation to identify that the incoming string is indeed a valid URL string (at the go backend `/submit` route), and if not valid return an error message to the frontend saying "Invalid URL used!"

    + A) Also, will need to add validation and string manipulations to ensure we are redundantly not re-processing URLs; i.e if already have `https://www.example.com` stored, if I then add `http://www.example.com` or `www.example.com` or `example.com`, it should all be treated as the same url (because it is the SAME url)

# MISSION 7 | Status: Pending | Details: Re-consider possible redundancy at the `status:` redis Key Pattern; since there is already the `url_task:` that has a status as well, and what actual use do you have for the `status/{uuid}` route anyway?

# MISSION 9 | Status: Pending | Details: Add some check in the llm-server to know when the ollama model has finished downloading and is ready for use. Then it should POST to a route in the go backend `/inform-model-loaded`, and add a websocket callback to update the frontend, informing it that it is now allowed to make requests and start (it should not allow users use the system while the model is still downloading - it should display a nice loading animation instead)

# MISSION 10 | Status: Pending | Details: What makes the `cache:` key actually behave like a cache? I think the caching mechanism is still a bit off

    - A) Set cached results to EXPIRE after a set time
    - B) Add a Postgres DB to store article results (model: {"url": str, "summary": str, "sentiment" str, "conversation": Array({"role": str, "content" str})})
    - C) Update the backed `/submit` route;
        + if not cached, first look if data exists in db, and if so retreive and cache it in Redis
        + if not cached and does not exist in DB, start new task
    - D) !!! Might need to add `"conversation": Array({"role": str, "content" str})` to the cache as well

# MISSION 11 | Status: Pending | Details: I want to add full support for having a conversation after getting result from article

    - A) Update frontend to allow engaging in a conversation for each url context;
        + Frontend should display a list of all cached and non-cached results (like chat gpt conversations history)
        + If something is retrived from non-cache (the DB), then cache it again with EXPIRE settings
    - B) Try adding another LLM step that attempts to clean the article text from unrelated text, since we are using a dirty web scrape
        + This means to simply add another prompt process with a prompt like `model.generate_text(text, "I am sending you a text of a scraped web page article. Please identify if there are unnecessary and unrelated characters, words or sentences that does")`
    - C) Add ChromaDB and embeddings logic to make each prompt have the additional context it needs for a better answer
    - D) For each prompt the user sends, that prompt needs to be used to search the vector db, to make each prompt have the additional context it needs for a better answer
