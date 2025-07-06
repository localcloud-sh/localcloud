# LocalCloud Export Test Plan

## Overview

This document outlines a comprehensive test plan for LocalCloud's export functionality covering all supported data stores. The test plan ensures data portability and migration capabilities to cloud services.

## Supported Export Formats

### 1. PostgreSQL Database Export
- **Format**: `.sql` file (via `pg_dump`)
- **Command**: `lc export db`
- **Compatible with**: AWS RDS, Google Cloud SQL, Azure Database, any PostgreSQL instance
- **Features**:
  - Full database schema and data
  - Includes custom types and extensions
  - Preserves table relationships and constraints

### 2. MongoDB Export
- **Format**: `.tar.gz` archive (via `mongodump`)
- **Command**: `lc export mongo`
- **Compatible with**: MongoDB Atlas, AWS DocumentDB, Google Cloud Firestore, any MongoDB instance
- **Features**:
  - All collections and documents
  - Indexes and metadata
  - BSON data preservation

### 3. MinIO Storage Export
- **Format**: `.tar.gz` archive with bucket structure
- **Command**: `lc export storage`
- **Compatible with**: AWS S3, Google Cloud Storage, Azure Blob Storage, any S3-compatible service
- **Features**:
  - All buckets and objects
  - Preserves directory structure
  - Object metadata where possible

### 4. Vector Database Export (NEW)
- **Format**: `.json` file with embeddings and metadata
- **Command**: `lc export vector [--collection=name]`
- **Compatible with**: Pinecone, Weaviate, Chroma, Qdrant, any pgvector instance
- **Features**:
  - Document IDs and content
  - Vector embeddings (1536-dimensional by default)
  - Collection information and metadata
  - Included SQL script for pgvector reimport

## Test Implementation

### Test File: `test-components/components/test-export.sh`

The comprehensive test suite includes:

#### 1. Environment Setup
- Validates LocalCloud project initialization
- Checks service availability (PostgreSQL, MongoDB, MinIO)
- Verifies required tools (`pg_dump`, `mongodump`, `psql`, `mongosh`)

#### 2. Sample Data Creation

**PostgreSQL Test Data:**
```sql
CREATE TABLE export_test_users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    age INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- 5 sample users inserted
```

**MongoDB Test Data:**
```javascript
db.export_test_products.insertMany([
    {
        "name": "Laptop Pro 16",
        "category": "Electronics", 
        "price": 2499.99,
        "stock": 15,
        "features": ["16GB RAM", "1TB SSD", "M2 Chip"]
    }
    // 4 sample products total
]);
```

**Vector Database Test Data:**
```sql
INSERT INTO localcloud.embeddings (document_id, content, embedding, metadata, collection_name) VALUES
    ('doc_1', 'Sample ML document', array_fill(random(), ARRAY[1536])::vector, 
     '{"type": "article", "category": "tech"}', 'export_test_embeddings'),
    -- 3 sample embeddings total
```

**MinIO Test Data:**
- Sample text files
- CSV data
- Binary files
- Nested directory structures

#### 3. Export Testing

**Individual Service Tests:**
- `test_postgres_export()` - Tests PostgreSQL export and validates SQL file
- `test_mongodb_export()` - Tests MongoDB export and validates archive
- `test_minio_export()` - Tests MinIO export and validates bucket structure
- `test_vector_export()` - Tests vector database export and validates JSON format

**Combined Export Test:**
- `test_export_all()` - Tests `lc export all` command
- Validates all export files are created correctly

#### 4. Export Validation

**File Integrity Checks:**
- SQL files: Validates PostgreSQL dump headers and SQL statements
- Archives: Tests gzip/tar integrity
- JSON files: Validates JSON structure and required fields
- Size validation: Ensures files contain expected amount of data

**Content Validation:**
- PostgreSQL: Verifies test tables are included in export
- MongoDB: Confirms collections are present in archive
- Vector DB: Validates embedding format and metadata structure
- MinIO: Checks bucket structure preservation

#### 5. Cleanup
- Removes test data from all services
- Cleans up temporary export files
- Ensures no test artifacts remain

## Enhanced Export Features

### Vector Database Export Structure

```json
{
  "export_info": {
    "exported_at": "2024-01-05T14:30:22Z",
    "source": "LocalCloud pgvector",
    "version": "1.0",
    "total_vectors": 150,
    "dimension": 1536
  },
  "collections": ["documents", "articles", "embeddings"],
  "embeddings": [
    {
      "id": "1",
      "document_id": "doc_123",
      "content": "Sample document text...",
      "embedding": [0.1, 0.2, 0.3, ...],
      "metadata": {"type": "article", "author": "user"},
      "collection": "documents",
      "created_at": "2024-01-05T10:00:00Z"
    }
  ],
  "import_script": "-- SQL script for pgvector import..."
}
```

### Collection-Specific Export
```bash
# Export specific collection only
lc export vector --collection=my-documents

# Export all collections (default)
lc export vector
```

## Migration Examples

### AWS Migration
```bash
# Export all data
lc export all --output=./aws-migration/

# Import to AWS services
psql $AWS_RDS_URL < localcloud-db-*.sql
mongorestore --uri $MONGODB_ATLAS_URI --archive=localcloud-mongo-*.tar.gz --gzip
aws s3 sync extracted-storage/ s3://my-bucket/
```

### Pinecone Migration
```bash
# Export vectors
lc export vector --output=./vectors.json

# Use Pinecone client to bulk upsert from JSON
python import_to_pinecone.py vectors.json
```

## Test Execution

### Run Individual Export Tests
```bash
# Test specific export functionality
./test-components/test-runner.sh -c export

# Run all tests including export
./test-components/test-runner.sh
```

### Prerequisites
1. LocalCloud project initialized (`lc setup`)
2. Services running (`lc start`)
3. Required tools installed:
   - `pg_dump` (PostgreSQL client)
   - `mongodump` (MongoDB tools)
   - `psql` (for validation)
   - `mongosh` (for validation)

## Expected Outcomes

### Successful Test Results
- ✅ All export commands execute without errors
- ✅ Export files created with expected formats
- ✅ File integrity validation passes
- ✅ Content validation confirms test data presence
- ✅ Export file sizes are reasonable
- ✅ All temporary data cleaned up

### Cloud Migration Ready
The exported files are production-ready for migration to:
- **PostgreSQL**: Any managed PostgreSQL service
- **MongoDB**: MongoDB Atlas, AWS DocumentDB, etc.
- **Storage**: AWS S3, GCS, Azure Blob Storage
- **Vector DB**: Pinecone, Weaviate, Chroma, Qdrant

## Error Scenarios Tested

1. **Missing Services**: Graceful handling when services aren't running
2. **Empty Databases**: Proper handling of empty datasets
3. **Large Datasets**: Performance with substantial data volumes
4. **Permission Issues**: File system permission problems
5. **Network Issues**: Database connection failures

This comprehensive test plan ensures LocalCloud's export functionality is robust, reliable, and ready for production use cases including cloud migration and data portability.