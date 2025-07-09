using System;
using Microsoft.EntityFrameworkCore.Migrations;
using Npgsql.EntityFrameworkCore.PostgreSQL.Metadata;

#nullable disable

namespace db_service.Migrations
{
    /// <inheritdoc />
    public partial class UpdateArticleResultModel : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "ConversationEntries");

            migrationBuilder.DropPrimaryKey(
                name: "PK_ArticleResults",
                table: "ArticleResults");

            migrationBuilder.DropColumn(
                name: "Id",
                table: "ArticleResults");

            migrationBuilder.AddColumn<Guid>(
                name: "Uuid",
                table: "ArticleResults",
                type: "uuid",
                nullable: false,
                defaultValue: new Guid("00000000-0000-0000-0000-000000000000"));

            migrationBuilder.AddPrimaryKey(
                name: "PK_ArticleResults",
                table: "ArticleResults",
                column: "Uuid");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropPrimaryKey(
                name: "PK_ArticleResults",
                table: "ArticleResults");

            migrationBuilder.DropColumn(
                name: "Uuid",
                table: "ArticleResults");

            migrationBuilder.AddColumn<int>(
                name: "Id",
                table: "ArticleResults",
                type: "integer",
                nullable: false,
                defaultValue: 0)
                .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn);

            migrationBuilder.AddPrimaryKey(
                name: "PK_ArticleResults",
                table: "ArticleResults",
                column: "Id");

            migrationBuilder.CreateTable(
                name: "ConversationEntries",
                columns: table => new
                {
                    Id = table.Column<int>(type: "integer", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    ArticleResultId = table.Column<int>(type: "integer", nullable: false),
                    Content = table.Column<string>(type: "text", nullable: false),
                    Role = table.Column<string>(type: "text", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_ConversationEntries", x => x.Id);
                    table.ForeignKey(
                        name: "FK_ConversationEntries_ArticleResults_ArticleResultId",
                        column: x => x.ArticleResultId,
                        principalTable: "ArticleResults",
                        principalColumn: "Id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "IX_ConversationEntries_ArticleResultId",
                table: "ConversationEntries",
                column: "ArticleResultId");
        }
    }
}
