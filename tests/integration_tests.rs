//! Integration tests for rigrun server
//!
//! These tests verify the full request flow works correctly by hitting the live server.
//! They are marked with #[ignore] so they don't run in CI without a server running.
//!
//! To run these tests:
//! 1. Start the rigrun server: rigrun
//! 2. Run tests with: cargo test --test integration_tests -- --ignored

use reqwest::Client;
use serde_json::{json, Value};

// =============================================================================
// Health Endpoint Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_health_endpoint() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let response = client.get("http://localhost:8787/health").send().await?;

    assert_eq!(response.status(), 200);

    let json: serde_json::Value = response.json().await?;
    assert_eq!(json["status"].as_str(), Some("ok"));
    assert!(json.get("version").is_some());

    Ok(())
}

// =============================================================================
// Models Endpoint Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_models_endpoint() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();
    let res = client
        .get("http://localhost:8787/v1/models")
        .send()
        .await?;

    assert_eq!(res.status(), 200);

    let json: Value = res.json().await?;
    assert!(json.is_object());

    let object = json.get("object").and_then(|v| v.as_str());
    assert_eq!(object, Some("list"));

    let data = json.get("data").and_then(|v| v.as_array());
    assert!(data.is_some());
    let data = data.unwrap();
    assert!(!data.is_empty());

    let model_ids: Vec<_> = data.iter()
        .filter_map(|model| model.get("id").and_then(|id| id.as_str()))
        .collect();

    assert!(model_ids.contains(&"auto"));
    assert!(model_ids.contains(&"local"));
    assert!(model_ids.contains(&"cloud"));

    Ok(())
}

// =============================================================================
// Cache Stats Endpoint Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_cache_stats_endpoint() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let response = client
        .get("http://localhost:8787/cache/stats")
        .send()
        .await?;

    assert_eq!(response.status(), 200);

    let json: serde_json::Value = response.json().await?;

    assert!(json.get("entries").is_some() && json["entries"].is_u64());
    assert!(json.get("total_lookups").is_some() && json["total_lookups"].is_u64());
    assert!(json.get("hits").is_some() && json["hits"].is_u64());
    assert!(json.get("misses").is_some() && json["misses"].is_u64());
    assert!(json.get("hit_rate_percent").is_some() && json["hit_rate_percent"].is_f64());

    let hit_rate = json["hit_rate_percent"].as_f64().unwrap_or(0.0);
    assert!(hit_rate >= 0.0 && hit_rate <= 100.0);

    Ok(())
}

// =============================================================================
// Query Routing Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_query_routing() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();
    let request_body = json!({
        "model": "auto",
        "messages": [{"role": "user", "content": "Hello, test query!"}]
    });

    let response = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&request_body)
        .send()
        .await?;

    assert_eq!(response.status(), 200);

    let json: serde_json::Value = response.json().await?;
    let choices = json["choices"].as_array()
        .ok_or_else(|| "No choices array")?;
    assert!(!choices.is_empty(), "Choices array is empty");

    let first_choice = &choices[0];
    let message = first_choice["message"].as_object()
        .ok_or_else(|| "No message in choice")?;
    assert_eq!(message["role"], "assistant");
    let content = message["content"].as_str()
        .ok_or_else(|| "No content string")?;
    assert!(!content.is_empty(), "Content is empty");

    let usage = json["usage"].as_object()
        .ok_or_else(|| "No usage object")?;
    let _prompt_tokens = usage["prompt_tokens"].as_i64()
        .ok_or_else(|| "No prompt_tokens")?;
    let _completion_tokens = usage["completion_tokens"].as_i64()
        .ok_or_else(|| "No completion_tokens")?;

    Ok(())
}

#[tokio::test]
#[ignore]
async fn test_query_routing_local() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();
    let request_body = json!({
        "model": "local",
        "messages": [{"role": "user", "content": "What is 2+2?"}]
    });

    let response = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&request_body)
        .send()
        .await?;

    assert_eq!(response.status(), 200);

    let json: serde_json::Value = response.json().await?;
    let choices = json["choices"].as_array()
        .ok_or_else(|| "No choices array")?;
    assert!(!choices.is_empty());

    let message = &choices[0]["message"];
    assert_eq!(message["role"], "assistant");
    assert!(!message["content"].as_str().unwrap().is_empty());

    Ok(())
}

// =============================================================================
// Stats Tracking Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_stats_tracking() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();

    // Get initial stats
    let initial_stats: serde_json::Value = client
        .get("http://localhost:8787/stats")
        .send()
        .await?
        .json()
        .await?;

    // Verify initial stats structure
    assert!(initial_stats["session"].is_object(), "session field must exist");
    assert!(initial_stats["today"].is_object(), "today field must exist");

    let initial_queries = initial_stats["session"]["queries"]
        .as_u64()
        .expect("session.queries must be a number");

    // Make a chat completion request
    let _completion_response = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&json!({
            "model": "local",
            "messages": [
                {"role": "user", "content": "Hello"}
            ]
        }))
        .send()
        .await?;

    // Get updated stats
    let updated_stats: serde_json::Value = client
        .get("http://localhost:8787/stats")
        .send()
        .await?
        .json()
        .await?;

    // Verify updated stats structure
    assert!(updated_stats["session"].is_object(), "session field must exist");
    assert!(updated_stats["today"].is_object(), "today field must exist");

    // Verify required session fields
    assert!(updated_stats["session"]["queries"].is_number(), "session.queries must exist");
    assert!(updated_stats["session"]["local_queries"].is_number(), "session.local_queries must exist");
    assert!(updated_stats["session"]["cloud_queries"].is_number(), "session.cloud_queries must exist");
    assert!(updated_stats["session"]["tokens_processed"].is_number(), "session.tokens_processed must exist");

    // Verify required today fields
    assert!(updated_stats["today"]["queries"].is_number(), "today.queries must exist");
    assert!(updated_stats["today"]["saved_usd"].is_number(), "today.saved_usd must exist");
    assert!(updated_stats["today"]["spent_usd"].is_number(), "today.spent_usd must exist");

    // Verify stats increased
    let updated_queries = updated_stats["session"]["queries"]
        .as_u64()
        .expect("session.queries must be a number");

    assert!(updated_queries > initial_queries, "queries should have increased");

    Ok(())
}

#[tokio::test]
#[ignore]
async fn test_stats_endpoint_structure() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();

    let response = client
        .get("http://localhost:8787/stats")
        .send()
        .await?;

    assert_eq!(response.status(), 200);

    let stats: serde_json::Value = response.json().await?;

    // Verify session stats structure
    assert!(stats["session"].is_object());
    assert!(stats["session"]["queries"].is_number());
    assert!(stats["session"]["local_queries"].is_number());
    assert!(stats["session"]["cloud_queries"].is_number());
    assert!(stats["session"]["tokens_processed"].is_number());

    // Verify today stats structure
    assert!(stats["today"].is_object());
    assert!(stats["today"]["queries"].is_number());
    assert!(stats["today"]["saved_usd"].is_number());
    assert!(stats["today"]["spent_usd"].is_number());

    Ok(())
}

// =============================================================================
// Cache Behavior Tests
// =============================================================================

#[tokio::test]
#[ignore]
async fn test_cache_hit_behavior() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();

    // Make the same request twice to test cache behavior
    let request_body = json!({
        "model": "auto",
        "messages": [{"role": "user", "content": "What is the capital of France?"}]
    });

    // First request - should go to local or cloud
    let response1 = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&request_body)
        .send()
        .await?;

    assert_eq!(response1.status(), 200);
    let json1: Value = response1.json().await?;
    let content1 = json1["choices"][0]["message"]["content"].as_str().unwrap();

    // Second request - might hit cache
    let response2 = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&request_body)
        .send()
        .await?;

    assert_eq!(response2.status(), 200);
    let json2: Value = response2.json().await?;
    let content2 = json2["choices"][0]["message"]["content"].as_str().unwrap();

    // Both should return valid responses
    assert!(!content1.is_empty());
    assert!(!content2.is_empty());

    Ok(())
}
