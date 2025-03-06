#!/usr/bin/env python3

import sys
import os
import datetime
from qq import QQClient, QQDatabaseError

def main():
    # Use command line argument if provided, otherwise use DATABASE_URL env var
    db_url = None
    if len(sys.argv) >= 2:
        db_url = sys.argv[1]
    elif "DATABASE_URL" not in os.environ:
        print("Usage: python example.py [postgres://user:password@localhost:5432/dbname]")
        print("Alternatively, set the DATABASE_URL environment variable.")
        sys.exit(1)
    
    # Create the client (will use provided db_url or DATABASE_URL env var)
    client = QQClient(db_url=db_url)
    
    try:
        # Add a job
        job = client.add_job(command="echo 'Hello from Python client!'")
        print(f"Added job: {job['id']}")
        
        # Add a scheduled job for 5 minutes in the future
        future_time = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=5)
        scheduled_job = client.add_job(
            command="echo 'This is a scheduled job'",
            queue_name="python-example",
            priority=2,
            schedule=future_time
        )
        print(f"Added scheduled job: {scheduled_job['id']}")
        
        # List all jobs
        all_jobs = client.list_jobs()
        print(f"\nFound {len(all_jobs)} jobs in all queues:")
        for j in all_jobs:
            print(f"  - {j['id']}: {j['command']} (queue: {j['queue']}, status: {j['status']})")
        
        # List queues
        queues = client.list_queues()
        print(f"\nAvailable queues: {', '.join(queues)}")
        
        # Get job details
        job_details = client.get_job(job_id=job['id'])
        print(f"\nJob details for {job['id']}:")
        print(f"  - Command: {job_details['command']}")
        print(f"  - Queue: {job_details['queue']}")
        print(f"  - Priority: {job_details['priority']}")
        print(f"  - Status: {job_details['status']}")
        print(f"  - Created at: {job_details['created_at']}")
        
        # Delete a job
        if input(f"\nDelete job {job['id']}? (y/n): ").lower() == 'y':
            client.delete_job(job_id=job['id'])
            print(f"Job {job['id']} deleted")
        
    except QQDatabaseError as e:
        print(f"Database error: {e}")
        if e.original_error:
            print(f"Original error: {e.original_error}")
    finally:
        # With this implementation, connections are automatically closed
        # client.close() call is kept for backward compatibility
        client.close()
        print("\nDone!")

if __name__ == "__main__":
    main()