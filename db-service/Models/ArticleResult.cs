using System.Collections.Generic;
using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace db_service.Models
{
    public class ArticleResult
    {
        [Key]
        [DatabaseGenerated(DatabaseGeneratedOption.None)]
        public Guid Uuid { get; set; }
        
        [Required]
        public string Url { get; set; } = string.Empty;
        
        [Required]
        public string Summary { get; set; } = string.Empty;
        
        [Required]
        public string Sentiment { get; set; } = string.Empty;
        
        public List<ConversationEntry> Conversation { get; set; } = new List<ConversationEntry>();
    }
} 