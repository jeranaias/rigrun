//! Server module for the local RAG Rust server
//!
//! This module handles HTTP server functionality including search endpoints,
//! embedding processing, and query validation.

use std::sync::Arc;
use thiserror::Error;

// ============================================================================
// Error Types
// ============================================================================

/// User-facing errors that can occur during server operations
#[derive(Debug, Error)]
pub enum UserError {
    #[error("Validation failed: {0}")]
    ValidationFailed(String),

    #[error("Search failed: {0}")]
    SearchFailed(String),

    #[error("Internal error: {0}")]
    Internal(String),
}

impl UserError {
    /// Create a validation failed error
    pub fn validation_failed(msg: impl Into<String>) -> Self {
        UserError::ValidationFailed(msg.into())
    }

    /// Create a search failed error
    pub fn search_failed(msg: impl Into<String>) -> Self {
        UserError::SearchFailed(msg.into())
    }
}

// ============================================================================
// Configuration Constants
// ============================================================================

/// Maximum allowed query length in bytes
const MAX_QUERY_LENGTH: usize = 100_000;

/// Maximum allowed embedding dimension
const MAX_EMBEDDING_DIM: usize = 4096;

// ============================================================================
// Input Validation
// ============================================================================

/// Validates a search query string
///
/// # Arguments
/// * `query` - The search query to validate
///
/// # Returns
/// * `Ok(())` if the query is valid
/// * `Err(UserError)` if validation fails
///
/// # Example
/// ```rust
/// let result = validate_search_query("my search query");
/// assert!(result.is_ok());
/// ```
pub fn validate_search_query(query: &str) -> Result<(), UserError> {
    if query.is_empty() {
        return Err(UserError::validation_failed("Query cannot be empty"));
    }
    if query.len() > MAX_QUERY_LENGTH {
        return Err(UserError::validation_failed(
            format!("Query too long: {} bytes (max {})", query.len(), MAX_QUERY_LENGTH)
        ));
    }
    Ok(())
}

/// Validates an embedding vector
///
/// # Arguments
/// * `embedding` - The embedding vector to validate
///
/// # Returns
/// * `Ok(())` if the embedding is valid
/// * `Err(UserError)` if validation fails
pub fn validate_embedding(embedding: &[f32]) -> Result<(), UserError> {
    // RROUTER-1 FIX: Check if embedding is empty before passing to search
    if embedding.is_empty() {
        return Err(UserError::validation_failed("Empty embedding vector"));
    }

    if embedding.len() > MAX_EMBEDDING_DIM {
        return Err(UserError::validation_failed(
            format!("Embedding dimension too large: {} (max {})", embedding.len(), MAX_EMBEDDING_DIM)
        ));
    }

    // Check for NaN or infinite values
    for (i, &val) in embedding.iter().enumerate() {
        if val.is_nan() {
            return Err(UserError::validation_failed(
                format!("Embedding contains NaN at index {}", i)
            ));
        }
        if val.is_infinite() {
            return Err(UserError::validation_failed(
                format!("Embedding contains infinite value at index {}", i)
            ));
        }
    }

    Ok(())
}

// ============================================================================
// Search Types
// ============================================================================

/// Search request parameters
#[derive(Debug, Clone)]
pub struct SearchRequest {
    pub query: String,
    pub top_k: usize,
    pub threshold: Option<f32>,
}

/// Search result item
#[derive(Debug, Clone)]
pub struct SearchResult {
    pub id: String,
    pub score: f32,
    pub content: String,
    pub metadata: Option<serde_json::Value>,
}

/// Search response
#[derive(Debug, Clone)]
pub struct SearchResponse {
    pub results: Vec<SearchResult>,
    pub query_time_ms: u64,
}

// ============================================================================
// Server Implementation
// ============================================================================

/// Vector search index trait
pub trait VectorIndex: Send + Sync {
    fn search(&self, embedding: &[f32], top_k: usize) -> Result<Vec<SearchResult>, UserError>;
}

/// Embedding service trait
pub trait EmbeddingService: Send + Sync {
    fn embed(&self, text: &str) -> Result<Vec<f32>, UserError>;
}

/// Main server structure
pub struct Server<I: VectorIndex, E: EmbeddingService> {
    index: Arc<I>,
    embedding_service: Arc<E>,
}

impl<I: VectorIndex, E: EmbeddingService> Server<I, E> {
    /// Create a new server instance
    pub fn new(index: Arc<I>, embedding_service: Arc<E>) -> Self {
        Self {
            index,
            embedding_service,
        }
    }

    /// Execute a search query
    ///
    /// This function validates the query, generates an embedding, and searches
    /// the vector index for similar documents.
    ///
    /// # Arguments
    /// * `request` - The search request containing query parameters
    ///
    /// # Returns
    /// * `Ok(SearchResponse)` with search results
    /// * `Err(UserError)` if validation or search fails
    pub fn search(&self, request: SearchRequest) -> Result<SearchResponse, UserError> {
        let start_time = std::time::Instant::now();

        // Validate the search query
        validate_search_query(&request.query)?;

        tracing::info!("Processing search query: {} chars", request.query.len());

        // Generate embedding for the query
        let embedding = self.embedding_service.embed(&request.query)?;

        // RROUTER-1 FIX: Validate embedding before passing to search
        // This prevents empty or invalid embeddings from causing downstream errors
        validate_embedding(&embedding)?;

        // Execute the search
        let results = self.index.search(&embedding, request.top_k)?;

        // Apply threshold filtering if specified
        let results = if let Some(threshold) = request.threshold {
            results.into_iter()
                .filter(|r| r.score >= threshold)
                .collect()
        } else {
            results
        };

        let query_time_ms = start_time.elapsed().as_millis() as u64;

        tracing::info!(
            "Search completed: {} results in {}ms",
            results.len(),
            query_time_ms
        );

        Ok(SearchResponse {
            results,
            query_time_ms,
        })
    }

    /// Execute a search with a pre-computed embedding
    ///
    /// # Arguments
    /// * `embedding` - Pre-computed embedding vector
    /// * `top_k` - Number of results to return
    ///
    /// # Returns
    /// * `Ok(SearchResponse)` with search results
    /// * `Err(UserError)` if validation or search fails
    pub fn search_by_embedding(
        &self,
        embedding: &[f32],
        top_k: usize,
    ) -> Result<SearchResponse, UserError> {
        let start_time = std::time::Instant::now();

        // RROUTER-1 FIX: Validate embedding before passing to search
        // Skip search if embedding is empty - this is a critical validation
        validate_embedding(embedding)?;

        tracing::info!("Processing embedding search: {} dimensions", embedding.len());

        // Execute the search
        let results = self.index.search(embedding, top_k)?;

        let query_time_ms = start_time.elapsed().as_millis() as u64;

        tracing::info!(
            "Embedding search completed: {} results in {}ms",
            results.len(),
            query_time_ms
        );

        Ok(SearchResponse {
            results,
            query_time_ms,
        })
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_search_query_empty() {
        let result = validate_search_query("");
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), UserError::ValidationFailed(_)));
    }

    #[test]
    fn test_validate_search_query_valid() {
        let result = validate_search_query("test query");
        assert!(result.is_ok());
    }

    #[test]
    fn test_validate_search_query_too_long() {
        let long_query = "a".repeat(MAX_QUERY_LENGTH + 1);
        let result = validate_search_query(&long_query);
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_embedding_empty() {
        // RROUTER-1: Test that empty embeddings are rejected
        let embedding: Vec<f32> = vec![];
        let result = validate_embedding(&embedding);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, UserError::ValidationFailed(_)));
    }

    #[test]
    fn test_validate_embedding_valid() {
        let embedding = vec![0.1, 0.2, 0.3, 0.4];
        let result = validate_embedding(&embedding);
        assert!(result.is_ok());
    }

    #[test]
    fn test_validate_embedding_with_nan() {
        let embedding = vec![0.1, f32::NAN, 0.3];
        let result = validate_embedding(&embedding);
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_embedding_with_infinity() {
        let embedding = vec![0.1, f32::INFINITY, 0.3];
        let result = validate_embedding(&embedding);
        assert!(result.is_err());
    }
}
