from urllib.parse import urlparse
from bs4 import BeautifulSoup
import requests
import logging


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
