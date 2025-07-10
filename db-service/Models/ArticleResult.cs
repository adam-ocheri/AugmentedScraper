using System.Collections.Generic;
using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;
using System.Text.Json.Serialization;

namespace db_service.Models
{
    public class ArticleResult
    {
        [Key]
        [DatabaseGenerated(DatabaseGeneratedOption.None)]
        [JsonPropertyName("uuid")]
        public Guid Uuid { get; set; }
        
        [Required]
        [JsonPropertyName("url")]
        public string Url { get; set; } = string.Empty;
        
        [Required]
        [JsonPropertyName("summary")]
        public string Summary { get; set; } = string.Empty;
        
        [Required]
        [JsonPropertyName("sentiment")]
        public string Sentiment { get; set; } = string.Empty;
        
        [JsonPropertyName("conversation")]
        public List<ConversationEntry> Conversation { get; set; } = new List<ConversationEntry>();
    }
} 