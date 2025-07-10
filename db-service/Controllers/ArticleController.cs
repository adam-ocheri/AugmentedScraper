using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using db_service.Data;
using db_service.Models;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using System;

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
    }
} 