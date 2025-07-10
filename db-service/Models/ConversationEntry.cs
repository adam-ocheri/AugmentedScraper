using System.ComponentModel.DataAnnotations;
using System.Text.Json.Serialization;

namespace db_service.Models
{
    public class ConversationEntry
    {
        public int Id { get; set; }
        
        [Required]
        public string Role { get; set; } = string.Empty;
        
        [Required]
        public string Content { get; set; } = string.Empty;
        
        public Guid ArticleResultId { get; set; }
        
        [JsonIgnore]
        public ArticleResult? ArticleResult { get; set; }
    }
} 