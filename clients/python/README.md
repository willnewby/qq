# QQ Python Client

A Python client library for interacting directly with the qq job queue database.

## Installation

```bash
pip install qq-client
```

Or from the source:

```bash
pip install -e .
```

## Requirements

This client requires:
- Python 3.7+
- psycopg 3.1.0+ (no connection pooling required)
- PostgreSQL database initialized with qq schema

## Usage

```python
from qq import QQClient

# Option 1: Initialize with explicit database URL
client = QQClient(db_url="postgres://user:password@localhost:5432/dbname")

# Option 2: Initialize using DATABASE_URL environment variable
import os
os.environ["DATABASE_URL"] = "postgres://user:password@localhost:5432/dbname"
client = QQClient()  # Will use DATABASE_URL environment variable

# Add a job to the default queue
job = client.add_job(command="echo 'Hello, world!'")
print(f"Added job with ID: {job['id']}")

# Add a job with custom parameters
import datetime
scheduled_time = datetime.datetime.now() + datetime.timedelta(hours=1)
job = client.add_job(
    command="python /path/to/script.py",
    queue_name="data-processing",
    priority=2,
    schedule=scheduled_time
)

# List all jobs
jobs = client.list_jobs()
print(f"Found {len(jobs)} jobs")

# List jobs in a specific queue
queue_jobs = client.list_jobs(queue_name="data-processing")
print(f"Found {len(queue_jobs)} jobs in data-processing queue")

# Get information about a specific job
job_info = client.get_job(job_id=job['id'])
print(f"Job status: {job_info['status']}")

# Delete a job
client.delete_job(job_id=job['id'])
print("Job deleted")

# List all queues
queues = client.list_queues()
print(f"Available queues: {', '.join(queues)}")

# Connections are automatically closed after each operation
# client.close() is available for backward compatibility
```

## API Reference

### `QQClient(db_url=None)`

Initialize a new client with the PostgreSQL database URL (or using the DATABASE_URL environment variable).

### Methods

- `add_job(command, queue_name="default", priority=1, schedule=None)` - Add a job to the queue
- `list_jobs(queue_name=None)` - List jobs in the queue
- `get_job(job_id)` - Get information about a specific job
- `delete_job(job_id)` - Delete a job from the queue
- `list_queues()` - List all available queues
- `close()` - No-op method kept for backward compatibility (connections are automatically closed)

## Error Handling

The client provides custom exceptions for handling errors:

```python
from qq import QQClient, QQDatabaseError

# Either specify the database URL explicitly
client = QQClient(db_url="postgres://user:password@localhost:5432/dbname")

# Or use the DATABASE_URL environment variable
# import os
# os.environ["DATABASE_URL"] = "postgres://user:password@localhost:5432/dbname"
# client = QQClient()

try:
    job = client.add_job(command="echo 'Hello, world!'")
except QQDatabaseError as e:
    print(f"Database error: {e}")
    if e.original_error:
        print(f"Original error: {e.original_error}")
```

## Schema Requirements

This client requires access to the River Queue schema in PostgreSQL. Make sure you have:

1. Initialized the database with `qq init`
2. Proper permissions to access the `river` schema tables
3. Connection to the correct database where the qq service is running

## Example Script

The package includes an example.py script demonstrating how to use all the client features:

```bash
# Option 1: Provide database URL as argument
python example.py postgres://user:password@localhost:5432/dbname

# Option 2: Use DATABASE_URL environment variable
export DATABASE_URL=postgres://user:password@localhost:5432/dbname
python example.py
```