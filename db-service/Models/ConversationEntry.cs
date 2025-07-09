namespace db_service.Models
{
    public class ConversationEntry
    {
        public int Id { get; set; }
        public string Role { get; set; }
        public string Content { get; set; }
        public Guid ArticleResultId { get; set; }
        public ArticleResult ArticleResult { get; set; }
    }
} 