using db_service.Data;
using Microsoft.EntityFrameworkCore;

var builder = WebApplication.CreateBuilder(args);

// Add services to the container.
// Learn more about configuring OpenAPI at https://aka.ms/aspnet/openapi
builder.Services.AddOpenApi();

// Register ArticleContext with PostgreSQL
builder.Services.AddDbContext<ArticleContext>(options =>
    options.UseNpgsql(builder.Configuration.GetConnectionString("DefaultConnection")));
builder.Services.AddControllers();

builder.Logging.AddConsole();

var app = builder.Build();
app.MapControllers();

// Custom log to verify logging is working
var logger = app.Services.GetRequiredService<ILogger<Program>>();
logger.LogInformation("Postgres-Dotnet service is starting up...");

// Apply migrations at startup
var retry = 0;
var maxRetries = 10;
while (retry < maxRetries)
{
    Console.WriteLine($"Trying to apply migrations... Attempt number {retry}");
    try
    {
        using (var scope = app.Services.CreateScope())
        {
            var db = scope.ServiceProvider.GetRequiredService<ArticleContext>();
            db.Database.Migrate();
        }
        break; // Success!
    }
    catch
    {
        retry++;
        Console.WriteLine($"Failed to apply migrations after {retry} retries");
        Thread.Sleep(5000); // Wait 5 seconds
    }
}
if (retry >= maxRetries)
{
    Console.WriteLine("Failed to apply migrations after 10 retries | EXITING");
    Environment.Exit(1);
}
// using (var scope = app.Services.CreateScope())
// {
//     var db = scope.ServiceProvider.GetRequiredService<db_service.Data.ArticleContext>();
//     db.Database.Migrate();
// }

// Configure the HTTP request pipeline.
if (app.Environment.IsDevelopment())
{
    app.MapOpenApi();
}


app.Run();

