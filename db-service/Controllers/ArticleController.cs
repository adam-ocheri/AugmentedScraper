using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using db_service.Data;
using db_service.Models;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using System;
using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace db_service.Controllers
{
    [ApiController]
    [Route("[controller]")]
    public class ArticleController : ControllerBase
    {
        private readonly ILogger<ArticleController> _logger;
        private readonly ArticleContext _context;

        public ArticleController(ArticleContext context, ILogger<ArticleController> logger)
        {
            _context = context;
            _logger = logger;
        }

        // GET /article?url=...
        [HttpGet]
        public async Task<ActionResult<ArticleResult>> GetByUrl([FromQuery] string url)
        {
            _logger.LogInformation("Received GET /article request for URL: {Url}", url);
            var article = await _context.ArticleResults
                .Include(a => a.Conversation)
                .FirstOrDefaultAsync(a => a.Url == url);
            if (article == null)
            {
                _logger.LogWarning("Article not found for URL: {Url}", url);
                return NotFound();
            }
            _logger.LogInformation("Article found for URL: {Url}", url);
            return article;
        }

        // GET /article/uuid/{uuid}
        [HttpGet("uuid/{uuid}")]
        public async Task<ActionResult<ArticleResult>> GetByUuid(string uuid)
        {
            _logger.LogInformation("Received GET /article/uuid request for UUID: {Uuid}", uuid);
            
            if (!Guid.TryParse(uuid, out Guid guidUuid))
            {
                _logger.LogWarning("Invalid UUID format: {Uuid}", uuid);
                return BadRequest("Invalid UUID format");
            }
            
            var article = await _context.ArticleResults
                .Include(a => a.Conversation)
                .FirstOrDefaultAsync(a => a.Uuid == guidUuid);
            if (article == null)
            {
                _logger.LogWarning("Article not found for UUID: {Uuid}", uuid);
                return NotFound();
            }
            _logger.LogInformation("Article found for UUID: {Uuid}", uuid);
            return article;
        }

        // POST /article
        [HttpPost]
        public async Task<ActionResult<ArticleResult>> PostArticle([FromBody] ArticleResult article)
        {
            try
            {
                _logger.LogInformation("Received POST /article request");
                
                if (article == null)
                {
                    _logger.LogError("Article is null - model binding failed");
                    return BadRequest("Article data is null or invalid format");
                }
                
                _logger.LogInformation("Article data: Uuid={Uuid}, Url={Url}, Summary={Summary}, Sentiment={Sentiment}", 
                    article.Uuid, article.Url, article.Summary, article.Sentiment);
                
                if (string.IsNullOrEmpty(article.Url))
                {
                    _logger.LogError("Article URL is null or empty");
                    return BadRequest("Article URL is required");
                }
                
                if (string.IsNullOrEmpty(article.Summary))
                {
                    _logger.LogError("Article Summary is null or empty");
                    return BadRequest("Article Summary is required");
                }
                
                if (string.IsNullOrEmpty(article.Sentiment))
                {
                    _logger.LogError("Article Sentiment is null or empty");
                    return BadRequest("Article Sentiment is required");
                }
                
                _logger.LogInformation("Received POST /article request for URL: {Url}", article.Url);
                _context.ArticleResults.Add(article);
                await _context.SaveChangesAsync();
                _logger.LogInformation("Article created for URL: {Url}", article.Url);
                return CreatedAtAction(nameof(GetByUrl), new { url = article.Url }, article);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error processing POST /article request");
                return BadRequest($"Error processing request: {ex.Message}");
            }
        }

        // PUT /article/conversation
        [HttpPut("conversation")]
        public async Task<ActionResult<ArticleResult>> UpdateConversation([FromBody] ConversationUpdateRequest request)
        {
            try
            {
                _logger.LogInformation("Received PUT /article/conversation request for UUID: {Uuid}", request.Uuid);
                
                if (request == null)
                {
                    _logger.LogError("Conversation update request is null");
                    return BadRequest("Conversation update data is null or invalid format");
                }
                
                if (string.IsNullOrEmpty(request.Uuid))
                {
                    _logger.LogError("Article UUID is null or empty");
                    return BadRequest("Article UUID is required");
                }
                
                if (!Guid.TryParse(request.Uuid, out Guid guidUuid))
                {
                    _logger.LogWarning("Invalid UUID format: {Uuid}", request.Uuid);
                    return BadRequest("Invalid UUID format");
                }
                
                var article = await _context.ArticleResults
                    .Include(a => a.Conversation)
                    .FirstOrDefaultAsync(a => a.Uuid == guidUuid);
                
                if (article == null)
                {
                    _logger.LogWarning("Article not found for UUID: {Uuid}", request.Uuid);
                    return NotFound($"Article with UUID {request.Uuid} not found");
                }
                
                // Clear existing conversation and add new entries
                article.Conversation.Clear();
                if (request.Conversation != null)
                {
                    foreach (var entry in request.Conversation)
                    {
                        article.Conversation.Add(new ConversationEntry
                        {
                            Role = entry.Role,
                            Content = entry.Content,
                            ArticleResultId = article.Uuid
                        });
                    }
                }
                
                await _context.SaveChangesAsync();
                _logger.LogInformation("Conversation updated for article UUID: {Uuid}", request.Uuid);
                return Ok(article);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error processing PUT /article/conversation request");
                return BadRequest($"Error processing request: {ex.Message}");
            }
        }
    }

    public class ConversationUpdateRequest
    {
        [JsonPropertyName("uuid")]
        public string Uuid { get; set; }
        [JsonPropertyName("conversation")]
        public List<ConversationEntry> Conversation { get; set; }
    }
} 