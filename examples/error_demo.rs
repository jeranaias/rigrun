//! Demo showing the improved error messages in rigrun.
//!
//! This example demonstrates the consistent error formatting across
//! Ollama and OpenRouter integrations.

use rigrun::local::{OllamaClient, OllamaError};
use rigrun::cloud::{OpenRouterClient, OpenRouterError};

fn main() {
    println!("=== rigrun Error Message Examples ===\n");

    // Example 1: Ollama Not Running
    println!("1. Ollama Not Running Error:");
    println!("{}", OllamaError::NotRunning("Cannot connect to Ollama at http://localhost:11434.".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 2: Ollama Timeout
    println!("2. Ollama Timeout Error:");
    println!("{}", OllamaError::Timeout("Generation request timed out after 300 seconds.".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 3: Model Not Found
    println!("3. Model Not Found Error:");
    println!("{}", OllamaError::ModelNotFound("llama3.2:latest".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 4: Ollama API Error
    println!("4. Ollama API Error:");
    println!("{}", OllamaError::ApiError("Ollama returned an error: invalid model format".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 5: OpenRouter Not Configured
    println!("5. OpenRouter Not Configured Error:");
    println!("{}", OpenRouterError::NotConfigured("API key is not set.".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 6: OpenRouter Auth Error
    println!("6. OpenRouter Authentication Error:");
    println!("{}", OpenRouterError::AuthError("Invalid API key.".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 7: OpenRouter Rate Limited
    println!("7. OpenRouter Rate Limit Error:");
    println!("{}", OpenRouterError::RateLimited("Too many requests.".to_string()));
    println!("\n{}\n", "=".repeat(80));

    // Example 8: Using ErrorBuilder
    println!("8. Custom Error Using ErrorBuilder:");
    let custom_error = rigrun::error::ErrorBuilder::new("Custom operation failed")
        .cause("Resource not available")
        .cause("Permission denied")
        .fix("Check resource availability")
        .fix("Verify permissions")
        .build();
    println!("{}", custom_error);
}
