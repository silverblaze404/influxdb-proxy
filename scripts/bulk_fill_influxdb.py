import requests
import time
import random
from datetime import datetime, timedelta
from concurrent.futures import ThreadPoolExecutor, as_completed
import json

# --- Configuration ---
# Your InfluxDB endpoint
INFLUXDB_URL = "http://localhost:8087"

# Database and measurement names (matching load_test.py)
DATABASE = "mydb"
MEASUREMENT = "xd"

# Data generation settings
TOTAL_DAYS = 2
DATA_POINTS_PER_HOUR = 60  # One data point per minute
BATCH_SIZE = 1000  # Number of points to send in each request
CONCURRENT_WORKERS = 10  # Number of concurrent upload threads

# Tag variations - these will be randomly selected for each data point
TAG_SETS = [
    {"region": "us-east-1", "datacenter": "dc1", "server": "web01", "environment": "prod"},
    {"region": "us-east-1", "datacenter": "dc1", "server": "web02", "environment": "prod"},
    {"region": "us-east-1", "datacenter": "dc2", "server": "web03", "environment": "prod"},
    {"region": "us-west-1", "datacenter": "dc3", "server": "web04", "environment": "staging"},
    {"region": "us-west-1", "datacenter": "dc3", "server": "web05", "environment": "staging"},
    {"region": "us-west-2", "datacenter": "dc4", "server": "web06", "environment": "dev"},
    {"region": "eu-west-1", "datacenter": "dc5", "server": "api01", "environment": "prod"},
    {"region": "eu-west-1", "datacenter": "dc5", "server": "api02", "environment": "prod"},
    {"region": "eu-central-1", "datacenter": "dc6", "server": "api03", "environment": "staging"},
    {"region": "ap-south-1", "datacenter": "dc7", "server": "db01", "environment": "prod"},
]

# Additional variable tags that can be added randomly
VARIABLE_TAGS = {
    "service_type": ["web", "api", "database", "cache", "queue"],
    "version": ["v1.0", "v1.1", "v1.2", "v2.0", "v2.1"],
    "cluster": ["primary", "secondary", "backup"],
    "instance_type": ["small", "medium", "large", "xlarge"],
}


def generate_data_point(timestamp):
    """
    Generate a single data point with timestamp, tags, and fields.
    
    Args:
        timestamp (datetime): The timestamp for this data point
    
    Returns:
        str: InfluxDB line protocol formatted string
    """
    # Select base tags from predefined sets
    base_tags = random.choice(TAG_SETS).copy()
    
    # Add some variable tags randomly (30% chance for each)
    for tag_key, tag_values in VARIABLE_TAGS.items():
        if random.random() < 0.3:  # 30% chance to include this tag
            base_tags[tag_key] = random.choice(tag_values)
    
    # Generate realistic field values
    fields = {
        "cpu_usage": round(random.uniform(10.0, 95.0), 2),
        "memory_usage": round(random.uniform(20.0, 85.0), 2),
        "disk_usage": round(random.uniform(15.0, 90.0), 2),
        "network_in": random.randint(1000, 50000),
        "network_out": random.randint(1000, 50000),
        "response_time": round(random.uniform(50.0, 2000.0), 2),
        "requests_per_sec": random.randint(10, 1000),
        "error_rate": round(random.uniform(0.0, 5.0), 2),
    }
    
    # Convert timestamp to nanoseconds since epoch
    timestamp_ns = int(timestamp.timestamp() * 1_000_000_000)
    
    # Build tag string
    tag_string = ",".join([f"{k}={v}" for k, v in base_tags.items()])
    
    # Build field string
    field_string = ",".join([f"{k}={v}" for k, v in fields.items()])
    
    # Return line protocol format: measurement,tags fields timestamp
    return f"{MEASUREMENT},{tag_string} {field_string} {timestamp_ns}"


def generate_batch_data(start_time, end_time, points_per_hour):
    """
    Generate a batch of data points between start_time and end_time.
    
    Args:
        start_time (datetime): Start timestamp
        end_time (datetime): End timestamp
        points_per_hour (int): Number of data points per hour
    
    Returns:
        list: List of line protocol formatted strings
    """
    data_points = []
    current_time = start_time
    interval = timedelta(hours=1) / points_per_hour
    
    while current_time <= end_time:
        data_points.append(generate_data_point(current_time))
        current_time += interval
    
    return data_points


def send_batch(batch_data):
    """
    Send a batch of data points to InfluxDB.
    
    Args:
        batch_data (list): List of line protocol formatted strings
    
    Returns:
        tuple: (success: bool, count: int, error_msg: str)
    """
    try:
        # Join all data points with newlines
        payload = "\n".join(batch_data)
        
        # Send POST request to InfluxDB write endpoint
        response = requests.post(
            f"{INFLUXDB_URL}/write",
            params={'db': DATABASE},
            data=payload,
            headers={'Content-Type': 'application/octet-stream'},
            timeout=30
        )
        
        if response.status_code == 204:  # InfluxDB returns 204 for successful writes
            return (True, len(batch_data), "")
        else:
            error_msg = f"HTTP {response.status_code}: {response.text[:200]}"
            return (False, 0, error_msg)
            
    except requests.exceptions.RequestException as e:
        return (False, 0, str(e))


def create_batches(data_points, batch_size):
    """
    Split data points into batches of specified size.
    
    Args:
        data_points (list): List of data points
        batch_size (int): Size of each batch
    
    Returns:
        list: List of batches
    """
    batches = []
    for i in range(0, len(data_points), batch_size):
        batches.append(data_points[i:i + batch_size])
    return batches


def main():
    """
    Main function to generate and upload bulk data to InfluxDB.
    """
    print("--- InfluxDB Bulk Data Fill Starting ---")
    print(f"Database: {DATABASE}")
    print(f"Measurement: {MEASUREMENT}")
    print(f"InfluxDB URL: {INFLUXDB_URL}")
    print(f"Duration: {TOTAL_DAYS} days")
    print(f"Data points per hour: {DATA_POINTS_PER_HOUR}")
    print(f"Batch size: {BATCH_SIZE}")
    print(f"Concurrent workers: {CONCURRENT_WORKERS}")
    print("-" * 50)
    
    # Calculate time range (last 2 days from now)
    end_time = datetime.now()
    start_time = end_time - timedelta(days=TOTAL_DAYS)
    
    print(f"Time range: {start_time.strftime('%Y-%m-%d %H:%M:%S')} to {end_time.strftime('%Y-%m-%d %H:%M:%S')}")
    
    # Generate all data points
    print("Generating data points...")
    all_data_points = generate_batch_data(start_time, end_time, DATA_POINTS_PER_HOUR)
    total_points = len(all_data_points)
    print(f"Generated {total_points:,} data points")
    
    # Create batches
    batches = create_batches(all_data_points, BATCH_SIZE)
    total_batches = len(batches)
    print(f"Split into {total_batches} batches")
    
    # Upload data using concurrent workers
    print("\nUploading data to InfluxDB...")
    successful_uploads = 0
    failed_uploads = 0
    total_uploaded_points = 0
    
    start_upload_time = time.perf_counter()
    
    with ThreadPoolExecutor(max_workers=CONCURRENT_WORKERS) as executor:
        # Submit all batch upload tasks
        future_to_batch = {executor.submit(send_batch, batch): i for i, batch in enumerate(batches)}
        
        # Process results as they complete
        for future in as_completed(future_to_batch):
            batch_index = future_to_batch[future]
            success, count, error_msg = future.result()
            
            if success:
                successful_uploads += 1
                total_uploaded_points += count
            else:
                failed_uploads += 1
                print(f"\n[Error] Batch {batch_index + 1} failed: {error_msg}")
            
            # Progress indicator
            completed = successful_uploads + failed_uploads
            progress = completed / total_batches * 100
            print(f"\rProgress: {completed}/{total_batches} batches ({progress:.1f}%) - "
                  f"Points uploaded: {total_uploaded_points:,}", end="")
    
    upload_time = time.perf_counter() - start_upload_time
    
    print("\n\n--- Bulk Data Fill Completed ---")
    print(f"Total time taken: {upload_time:.2f} seconds")
    print(f"Total batches: {total_batches}")
    print(f"Successful uploads: {successful_uploads}")
    print(f"Failed uploads: {failed_uploads}")
    print(f"Total points uploaded: {total_uploaded_points:,}")
    print(f"Points per second: {total_uploaded_points / upload_time:.0f}")
    
    if failed_uploads > 0:
        print(f"\n⚠️  {failed_uploads} batch(es) failed. Check the error messages above.")
    else:
        print("\n✅ All batches uploaded successfully!")
    
    print(f"\nYou can now run load_test.py to test querying this data.")


if __name__ == "__main__":
    main()
