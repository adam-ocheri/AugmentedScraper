# Augmented Scraper - Content Filtering && Summarization System

## Setup

To run the project, simply execute `$ docker-compose up -d`.

Before submitting a new request, please wait until the Ollama finished downloading and loading the model (Temporary, until `Mission 9` is completed and the frontend is notified and displaying loading animation until then)
You can use `$ docker logs llm-service -f` and see when the download and loading is finished (downloading only once then stored in persistent volume)

Note: This project uses an image of Ollama to perform model inference. If you are having issues running the ollama container, please make sure you have the NVIDIA Container Toolkit: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html

- Developed and tested on machine specs: Windows 11 64-bit, Intel i9 9900-k, RTX 3070 8 GB VRAM, 64 GB RAM

If you are encoutering problems, feel free to reach out or even add an issue in this repo.

## Project Details and Architecture

This project lays solid foundations for a content summarization and filtering system:

- It uses Redis to cache transient data, queue, and dispatcher to trigger events across the services (PubSub)
- Using Postgres as the "single source of truth" - if no data cached will trigger call to db to get data and cache again (wrapped in a Dotnet 9 app)
- Using Go as the main Backend API - main handler of traffic across the containers (add new articles, fetch existing, push WS events)
- Using python FastAPI service as a wrapper around Ollama, acting as the project's LLM service
- Using ChromaDB to store vector data of articles for Retrival Augemented Generation, improving conversation context

For some additional visualisation, see the attached `AugementedScraper.drawio` file

## Features & Flow

- Allows users to safely input a URL to process (validated as public https address && valid page returning 200)
- The new requested task data is added to a Redis queue which is automatically absorbed by the python llm server
- The llm server triggers ollama to provide a summary and sentiment for the article, as well as embed the article in vector format for later retreival
- Once the LLM finished processing the article, it will cache the result in Redis and send dispatch which is then absorbed by the Go backend, saving the new data in the Postgres database
- Finally, after the new result is stored in the DB, the backend sends a websocket event which is absorbed by the frontend, updating the article results and history
- Finally 2.0, when the frontend is updated with the new article data, it exposes a chat feature

When submitting/fetching data:

- First checking if it is still cached - if yes, return cache data
- If not cached, check if data exists in db - if exists return data and cache with EXPIRE TTL again
- If not exists in the db (and submitting), we can create new article (and then the flow above again)
