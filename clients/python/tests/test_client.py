import os
import unittest
import time
import subprocess
import tempfile
import shutil
import psycopg
from qq import QQClient

class TestQQClient(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        """Set up a PostgreSQL container for testing"""
        # Check if Docker is available
        try:
            subprocess.run(["docker", "info"], check=True, capture_output=True)
        except (subprocess.CalledProcessError, FileNotFoundError):
            raise unittest.SkipTest("Docker not available, skipping integration test")
        
        # Create a unique container name
        cls.container_name = f"qq-python-test-{int(time.time())}"
        
        # Start PostgreSQL container
        docker_args = [
            "docker", "run", "--rm", "-d",
            "-e", "POSTGRES_PASSWORD=postgres",
            "-e", "POSTGRES_USER=postgres",
            "-e", "POSTGRES_DB=qq_test",
            "-p", "5432", # Let Docker assign a random port
            "--name", cls.container_name,
            "postgres:14-alpine"
        ]
        
        subprocess.run(docker_args, check=True)
        
        # Get the assigned port
        port_cmd = ["docker", "port", cls.container_name, "5432/tcp"]
        port_output = subprocess.run(port_cmd, check=True, capture_output=True, text=True).stdout
        
        # Extract port from output like "0.0.0.0:49153"
        port = port_output.strip().split(':')[1]
        
        # Wait for PostgreSQL to be ready
        time.sleep(3)
        
        # Store connection URL
        cls.db_url = f"postgres://postgres:postgres@localhost:{port}/qq_test"
        
        # Build and initialize db
        temp_dir = tempfile.mkdtemp()
        cls.temp_dir = temp_dir
        
        try:
            # Build qq binary
            subprocess.run(["go", "build", "-o", f"{temp_dir}/qq", "."],
                          cwd=os.path.abspath(os.path.join(os.path.dirname(__file__), "../../..")),
                          check=True)
            
            # Initialize database
            subprocess.run([f"{temp_dir}/qq", "init", "--db-url", cls.db_url], 
                          check=True)
        except Exception as e:
            # Clean up on failure
            shutil.rmtree(temp_dir)
            cls.tearDownClass()
            raise unittest.SkipTest(f"Failed to initialize test environment: {e}")
    
    @classmethod
    def tearDownClass(cls):
        """Clean up resources"""
        try:
            # Stop PostgreSQL container
            subprocess.run(["docker", "stop", cls.container_name], check=False)
        except:
            pass
        
        # Remove temporary directory
        if hasattr(cls, 'temp_dir'):
            shutil.rmtree(cls.temp_dir, ignore_errors=True)
    
    def setUp(self):
        """Create client before each test"""
        self.client = QQClient(db_url=self.__class__.db_url)
    
    def test_add_job(self):
        """Test adding a job to the queue"""
        job = self.client.add_job(command="echo 'test from python'")
        self.assertIsNotNone(job.get("id"))
        self.assertEqual(job.get("command"), "echo 'test from python'")
    
    def test_list_jobs(self):
        """Test listing jobs"""
        # Add a job first
        self.client.add_job(command="echo 'test list jobs'")
        
        # List jobs
        jobs = self.client.list_jobs(limit=10)
        self.assertGreaterEqual(len(jobs), 1)
    
    def test_get_job(self):
        """Test getting a specific job"""
        # Add a job
        job = self.client.add_job(command="echo 'test get job'")
        job_id = job.get("id")
        
        # Get the job by ID
        retrieved_job = self.client.get_job(job_id)
        self.assertEqual(retrieved_job.get("id"), job_id)
        self.assertEqual(retrieved_job.get("command"), "echo 'test get job'")

if __name__ == "__main__":
    unittest.main()