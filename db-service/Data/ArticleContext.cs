using Microsoft.EntityFrameworkCore;
using db_service.Models;

namespace db_service.Data
{
    public class ArticleContext : DbContext
    {
        public ArticleContext(DbContextOptions<ArticleContext> options) : base(options) { }

        public DbSet<ArticleResult> ArticleResults { get; set; }
        public DbSet<ConversationEntry> ConversationEntries { get; set; }

        protected override void OnModelCreating(ModelBuilder modelBuilder)
        {
            modelBuilder.Entity<ArticleResult>()
                .HasMany(a => a.Conversation)
                    .WithOne(c => c.ArticleResult)
                    .HasForeignKey(c => c.ArticleResultId);
            modelBuilder.Entity<ArticleResult>()
                .Navigation(a => a.Conversation);
        }
    }
} 