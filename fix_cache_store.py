#!/usr/bin/env python3
import re

# Read the file
with open(r'C:\rigrun\src\server\mod.rs', 'r', encoding='utf-8') as f:
    content = f.read()

# Define the old pattern to replace
old_pattern = r'''    // Store response in cache for future hits
    let mut cache = state\.cache\.write\(\)\.await;
    cache\.store_response\(
        cache_key,
        response_text\.clone\(\),
        actual_tier,
        prompt_tokens \+ completion_tokens,
    \)\.await;'''

# Define the new replacement
new_code = '''    // Store response in cache for future hits
    // CRITICAL FIX: Use pre-generated embedding to avoid holding write lock during embedding generation
    let mut cache = state.cache.write().await;
    if let Some(emb) = embedding {
        // Use store_with_embedding to avoid regenerating the embedding
        cache.store_with_embedding(
            cache_key,
            emb,
            response_text.clone(),
            actual_tier,
            prompt_tokens + completion_tokens,
        );
    } else {
        // Fallback to regular store if embedding generation failed earlier
        cache.store_response(
            cache_key,
            response_text.clone(),
            actual_tier,
            prompt_tokens + completion_tokens,
        ).await;
    }'''

# Perform the replacement
new_content = re.sub(old_pattern, new_code, content)

# Write the result back
with open(r'C:\rigrun\src\server\mod.rs', 'w', encoding='utf-8') as f:
    f.write(new_content)

print("File modified successfully!")
