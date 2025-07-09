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
        public string Url { get; set; }
        public string Summary { get; set; }
        public string Sentiment { get; set; }
        public List<ConversationEntry> Conversation { get; set; } = new List<ConversationEntry>();
    }
} 