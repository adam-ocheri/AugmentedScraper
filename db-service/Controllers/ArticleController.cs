using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using db_service.Data;
using db_service.Models;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

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
            _logger.LogInformation("Received POST /article request for URL: {Url}", article.Url);
            _context.ArticleResults.Add(article);
            await _context.SaveChangesAsync();
            _logger.LogInformation("Article created for URL: {Url}", article.Url);
            return CreatedAtAction(nameof(GetByUrl), new { url = article.Url }, article);
        }
    }
} 